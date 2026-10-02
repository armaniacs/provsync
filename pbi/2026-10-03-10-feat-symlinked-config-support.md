# PBI: シンボリックリンクされた設定ファイルの扱い

## ユーザーストーリー
dotfiles をシンボリックリンクで管理している開発者として、provsync の書き込みでリンクが壊れないでほしい、なぜなら chezmoi や stow などで `~/.config/...` を管理していて、リンクが通常ファイルに置き換わると管理リポジトリへ変更が反映されなくなるから

## 優先度
- 順位: 10 / 19
- RICEスコア: 5（Reach=5 / Impact=2 / Confidence=50% / Effort=1）
- 根拠: 書き込みは一時ファイル + `os.Rename` による置換で、コード上に `Lstat` / `EvalSymlinks` の使用が見つからない。リンク先ではなくリンク自体が置換される可能性が高いが、再現確認が済んでいないため確信度は 50%。データ破損ではなく管理の断絶なので中位

## 設計方針
- まず再現テストを書いて現状の挙動を確認する（確認前に仕様を決めない）
- 方針案: 書き込み先は `filepath.EvalSymlinks` でリンク先の実体を解決し、そこへ atomic に書く。リンクは維持する。バックアップは実体の内容を記録する
- リンク切れ・循環は明確なエラーにする

## BDD受け入れシナリオ
Scenario: リンクを維持したままリンク先を更新する
  Given `opencode.json` が dotfiles リポジトリ内のファイルへのシンボリックリンクである
  When  `provsync push opencode --write` を実行する
  Then  リンクはそのまま残り、リンク先のファイルが更新される

Scenario: undo もリンクを壊さない
  Given 上の push の操作が記録されている
  When  `provsync undo --write` を実行する
  Then  リンクは維持され、リンク先が元の内容に戻る

Scenario: リンク切れは書き込まずにエラー
  Given 設定パスが存在しないファイルへのリンクである
  When  `provsync push opencode --write` を実行する
  Then  何も書き込まれず、リンク切れである旨のエラーで終了する

## 受け入れ基準
- [x] 現状挙動を再現するテストを先に追加し、結果を記録する
- [x] 書き込み・バックアップ・復元のすべてで実体パスを解決する
- [x] `list` / `status` はリンクであることを表示する
- [x] atomic 書き込み（同一ディレクトリの一時ファイル + rename）を保つ。リンク先が別ファイルシステムでも一時ファイルをリンク先と同じディレクトリに作る

## テスト戦略
- E2E: `t.TempDir()` 内にリンクを作り push / undo の前後を検証
- 統合: 3 シナリオ
- 単体: パス解決（相対リンク、多段リンク、循環）

## 見積もり
3 ポイント（要チームでの見積もり）

## Definition of Done
- [x] 全BDDシナリオが自動テストとして実装されパスする
- [x] `make check` がパスする
- [x] README に dotfiles 管理との併用を追記、CHANGELOG 更新済み

## 実装ガイド（この順に実施。先に [00-implementation-guide.md](00-implementation-guide.md) を読む）

### 先に読むファイル
`internal/fsutil/fsutil.go`（`WriteFileAtomic`）、`internal/backup/backup.go` の `capture` / `restoreFile`、`cli_test.go` の `setup`。

### 事実の整理（コード調査済み）
`WriteFileAtomic` は同じディレクトリに一時ファイルを作り `os.Rename(tmp, path)` で置換する。`path` がシンボリックリンクだと、リンクそのものが通常ファイルに置き換わり、リンク先は更新されない。権限は `os.Stat`（リンクを辿る）から取るので権限だけリンク先のものになる。バックアップの読み取り（`os.Stat` / `os.ReadFile`）はリンクを辿るので問題ない。

### 手順（テストを先に）
1. 再現テストを先に書く。`cli_test.go`:

```go
func TestPushKeepsSymlink(t *testing.T) {
	f := setup(t)
	mustRun(t, f.root, "pull", "kilocode", "--write")
	real := filepath.Join(t.TempDir(), "opencode.json")
	data, _ := os.ReadFile(f.opencode)
	if err := os.WriteFile(real, data, 0o644); err != nil { t.Fatal(err) }
	if err := os.Remove(f.opencode); err != nil { t.Fatal(err) }
	if err := os.Symlink(real, f.opencode); err != nil { t.Fatal(err) }

	mustRun(t, f.root, "push", "opencode", "--write")

	info, err := os.Lstat(f.opencode)
	if err != nil { t.Fatal(err) }
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("symlink was replaced by a regular file")
	}
	got, _ := os.ReadFile(real)
	if !strings.Contains(string(got), "llm-01") {
		t.Error("link target was not updated")
	}
}
```
   修正前に実行して失敗する（リンクが通常ファイルに置き換わる）ことを確認する。失敗しなければ調査を報告して止まる。
2. 修正は `fsutil.WriteFileAtomic` の先頭で実体パスを解決する。

```go
func WriteFileAtomic(path string, data []byte) error {
	resolved, err := resolveWritePath(path)
	if err != nil {
		return err
	}
	path = resolved
	// 以降は従来どおり。dir := filepath.Dir(path) は解決後のパスから取る
}

// resolveWritePath はシンボリックリンクを辿った実体のパスを返す。
// 一時ファイルをリンク先と同じディレクトリに作るため、解決後のパスで書き込む。
func resolveWritePath(path string) (string, error) {
	li, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return path, nil
		}
		return "", err
	}
	if li.Mode()&os.ModeSymlink == 0 {
		return path, nil
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("シンボリックリンクのリンク先を解決できません (%s): %w", path, err)
	}
	return resolved, nil
}
```
   （`fmt` を import する）。
3. `fsutil_test.go` に追加: リンク切れで error、相対リンクでも実体が更新される、多段リンクでも実体が更新される。
4. `cli_test.go` に追加: `TestUndoKeepsSymlink`（push 後に `undo --write` してもリンクが残り、リンク先が元の内容に戻る）。`restoreFile` は `WriteFileAtomic` と `os.Chmod` を使うので修正不要のはず。通らなければ `restoreFile` を読んで報告する。
5. 任意: `list` に、リンクなら ` (symlink → 実体)` を出す（`os.Lstat` と `os.Readlink`）。

### 注意
- 一時ファイルは必ず解決後のパスのディレクトリに作る（別ファイルシステムだと `rename` が失敗する）。
- 循環リンクは `EvalSymlinks` がエラーを返すので、そのままエラーにする。
- README に「dotfiles 管理のシンボリックリンクは維持される」と追記。
