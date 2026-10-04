# PBI: doctor コマンド（環境診断）

## ユーザーストーリー
つまずいた利用者として、`provsync doctor` で設定の問題を一覧で診断したい、なぜなら「動かない」ときにファイルの場所・構文・環境変数・権限を個別に調べるのは手間だから

## 優先度
- 順位: 16 / 19
- RICEスコア: 2（Reach=6 / Impact=1 / Confidence=50% / Effort=1.5）
- 根拠: init（05）と設定パス表示（02）で初回のつまずきの大半は防げる。残る診断項目の需要は不明で確信度が低い。通信を伴う疎通確認は 18 の `check` に委ね、ここでは通信しない

## BDD受け入れシナリオ
Scenario: 問題がなければ全項目が OK
  Given 中央設定と各ツール設定が正常である
  When  `provsync doctor` を実行する
  Then  項目ごとに OK が表示され、終了コードは 0 である

Scenario: 構文エラーを具体的に指摘する
  Given kilocode の設定ファイルに JSON の構文エラーがある
  When  `provsync doctor` を実行する
  Then  該当ファイルと行・桁、修正の手がかりが表示され、終了コードは 1 である

Scenario: apiKeyEnv の環境変数が未設定
  Given 中央設定の provider が `apiKeyEnv` を参照し、その環境変数が未設定である
  When  `provsync doctor` を実行する
  Then  未設定の変数名が警告として表示され、値は表示されない

## 受け入れ基準
- [x] 診断項目: パス解決、存在、構文、スキーマ版、`apiKeyEnv` の設定有無、ファイル権限（08）、in-file の秘密（有無のみ）
- [x] 通信しない。ファイルを書かない
- [x] 各項目に「次にやること」を 1 行で添える
- [x] 診断の再利用部品は既存の store / adapter を使い、判定を重複させない

## テスト戦略
- E2E: 壊れた設定を置いた HOME で実行
- 統合: 3 シナリオ
- 単体: 各診断項目

## 見積もり
3 ポイント（要チームでの見積もり）

## Definition of Done
- [x] 全BDDシナリオが自動テストとして実装されパスする
- [x] `make check` がパスする
- [x] README / CHANGELOG 更新済み

## 実装ガイド（この順に実施。先に [00-implementation-guide.md](../00-implementation-guide.md) を読む）

### 先に読むファイル
`cmdStatus`、`cmdList`、`adapter.Get` / `Pull`、`store.Load`、08 の `warnLoosePerm`。02・05・08 が済んでいること。

### 手順
1. `Run` に `case "doctor": return cmdDoctor(opts)` と `usage` の 1 行を足す。
2. 診断結果の型と出力:

```go
type check struct {
	Name   string
	Status string // "OK" / "警告" / "NG"
	Detail string // 次にやること
}
```
   出力形式: `[OK] 名前` / `[警告] 名前: 詳細` / `[NG] 名前: 詳細`。末尾に集計。`NG` が 1 件以上なら `fmt.Errorf("診断で問題が見つかりました (NG %d 件)", n)` を返す（終了コード 1）。警告のみなら nil。
3. 診断項目（この順。各項目は小さな関数 `func checkXxx(...) []check` にする）:
   - パス解決: `root.CentralConfigPath()` を表示するだけ（`OK`）。
   - 中央設定: 無ければ `警告`（`provsync init <tool> --write を実行してください`）。あれば `store.Load` で構文を確認、失敗は `NG`（エラー文をそのまま詳細に）。
   - 各ツール設定: 無ければ `警告`。あれば `a.Pull()` で構文・読み取りを確認、失敗は `NG`、取り込み時の警告（秘密らしいキー）は `警告`（キー名のみ。値は出さない）。
   - `apiKeyEnv`: 中央設定の各 provider で `APIKeyEnv != ""` のものについて `os.LookupEnv(name)` の有無だけ確認。未設定は `警告`（変数名のみ。値は読んでも出さない）。
   - 権限: 中央設定と `root.StateDir()` に 08 の判定を使う。
4. 通信しない。ファイルを書かない。
5. テスト: 正常な環境で全 OK で nil、kilocode の JSON を壊すと NG とエラー、`apiKeyEnv` が指す環境変数が未設定で警告（`t.Setenv` で制御）、秘密らしいキーがあっても出力に値が出ない。

### 注意
- 判定ロジックを新規に書かず、既存の `store.Load` / `Pull` / 08 の関数を呼ぶ。
- ネットワークの疎通確認はここに入れない（18 の `check` の仕事）。
