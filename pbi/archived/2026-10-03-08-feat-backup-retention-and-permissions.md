# PBI: バックアップの保持設定・掃除コマンド・ファイル権限の確認

## ユーザーストーリー
provsync の利用者として、バックアップの保持数と保存先の権限を把握・制御したい、なぜならバックアップには in-file の秘密を含む設定の複製が入り、無制限に残らず、他ユーザーから読めない状態であってほしいから

## 優先度
- 順位: 08 / 19
- RICEスコア: 6.4（Reach=4 / Impact=1 / Confidence=80% / Effort=0.5）
- 根拠: 現状、保持数は定数 `MaxOperations` 固定で調整できない。バックアップディレクトリは 0700 で作成されるが、バックアップ内のファイルと中央設定の権限は実装時に確認が必要。リスク軽減効果により同点の release（09）より先

## BDD受け入れシナリオ
Scenario: 保持数を設定できる
  Given 環境変数または設定で保持数が 5 に指定されている
  When  6 回目の `--write` を実行する
  Then  最も古い操作のバックアップが削除され、履歴は 5 件になる

Scenario: 不要なバックアップを掃除する
  Given 履歴が複数ある
  When  `provsync undo --prune --keep 2` を実行する
  Then  新しい 2 件を残して削除され、削除件数が表示される

Scenario: 権限が緩いと警告する
  Given バックアップまたは中央設定が他ユーザーから読める権限である
  When  `provsync status` を実行する
  Then  権限の警告と `chmod` の案内が表示される

Scenario: 新規作成されるファイルは所有者のみ読み書き可能
  Given 中央設定が存在しない
  When  `provsync init kilocode --write` を実行する
  Then  中央設定は 0600、親ディレクトリは 0700 で作成される

## 受け入れ基準
- [x] 保持数の既定値は現状の `MaxOperations` を維持する（既定の挙動は変えない）
- [x] 既存ファイルの権限は変更せず、新規作成分のみ 0600 / 0700 にする
- [x] 現状の権限を最初に調査し、結果をこの PBI のメモに残す
- [x] 掃除は undo 用の索引と実体の両方を整合させて削除する

## テスト戦略
- E2E: 実ファイルで権限ビットを `os.Stat` で検証
- 統合: `cli.Run --root <tmpdir>` で 4 シナリオ
- 単体: 保持数の解釈（不正値・0・負数）

## 見積もり
2 ポイント（要チームでの見積もり）

## Definition of Done
- [x] 全BDDシナリオが自動テストとして実装されパスする
- [x] `make check` がパスする
- [x] README / CHANGELOG 更新済み

## 実装ガイド（この順に実施。先に [00-implementation-guide.md](00-implementation-guide.md) を読む）

### 先に読むファイル
`internal/backup/backup.go` 全体（`MaxOperations`、`prune`、`Record`、`capture`）、`internal/fsutil/fsutil.go` と `fsutil_test.go`、`cmdUndo`、`cmdStatus`。

### 事実の整理（調査済み）
- 保持数は定数 `MaxOperations = 20`。
- `fsutil.WriteFileAtomic` は、新規ファイルを 0644、新規ディレクトリを 0755 で作る（既存ファイルは権限を引き継ぐ）。中央設定と `index.json` は新規作成されるので 0644 になる。
- バックアップ操作ディレクトリは 0700、中にコピーされるファイルは元ファイルの権限。

### 手順
1. 保持数を設定可能にする:
   - `backup.Store` に `max int` フィールドを足し、`New(stateDir)` は `max: MaxOperations` とする。`func (s *Store) SetMax(n int)` を足す。
   - `prune` の比較を `MaxOperations` → `s.max` に変える。
   - `cli` 側で環境変数 `PROVSYNC_KEEP` を読む補助関数 `keepFromEnv() (int, error)` を作る（未設定は既定値。`strconv.Atoi` で整数化し、1 未満・非数はエラー `PROVSYNC_KEEP は 1 以上の整数で指定してください`）。`backup.New(root.StateDir())` を作る 2 か所で `SetMax` する。
2. 掃除: `backup.Store` に `Prune(keep int) (removed int, err error)` を足す（索引を読み、先頭 `keep` 件を残し、残りの実体ディレクトリを `os.RemoveAll`、索引を保存）。既存の `prune` の中身を共通化して使う。`options` に `prune bool` と `keep int`（既定 20）を足し、`cmdUndo` の先頭で `o.prune` なら `Prune` を呼んで `削除: N 件` を出して終わる。
3. 新規ファイルの権限: `fsutil.WriteFileAtomic` の既定を、新規ファイル 0600・新規ディレクトリ 0700 に変える（`mode := os.FileMode(0o600)`、`os.MkdirAll(dir, 0o700)`）。既存ファイルは今までどおり権限を引き継ぐ。`fsutil_test.go` に新規ファイルが 0600 になるテストを足す。`TestWriteFileAtomicCreatesDir` が権限を検査していれば期待値を更新する。ただし MkdirAll は既存の親ディレクトリの権限を変えない点に注意（`~/.config` 自体は変わらない）。
4. 権限警告: `cli` に `warnLoosePerm(out io.Writer, path string)` を作る（`os.Stat` → `info.Mode().Perm()&0o077 != 0` なら `警告: 権限が緩い (%04o): %s  chmod 600 %s` 形式）。`cmdStatus` から、中央設定のファイルに対して呼ぶ（存在するときのみ）。状態ディレクトリ `root.StateDir()` は `&0o077 != 0` なら `chmod 700` を案内。
5. テスト: 保持数 3 にして 4 回 `--write` すると `undo --list` が 3 件（`t.Setenv("PROVSYNC_KEEP", "3")`）、`undo --prune --keep 1` で 1 件に減る、中央設定を `os.Chmod(path, 0o644)` にすると `status` に `権限が緩い`、`init`/`pull --write` で作られた中央設定が 0600、不正な `PROVSYNC_KEEP=abc` がエラー。

### 注意
- 既定の保持数 20 を変えない。
- 既存ファイルの権限は変更しない（新規作成分のみ 0600/0700）。ユーザーの tool 設定の権限を絞ってはいけない。
- バックアップの復元（`restoreFile`）は `WriteFileAtomic` の後で `os.Chmod(f.Path, f.Mode)` する既存挙動のまま。
