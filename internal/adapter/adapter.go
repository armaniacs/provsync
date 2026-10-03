// Package adapter はツール固有の設定形式とカノニカル表現を相互変換する。
package adapter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/armaniacs/provsync/internal/fsutil"
	"github.com/armaniacs/provsync/internal/i18n"
	"github.com/armaniacs/provsync/internal/jsonc"
	"github.com/armaniacs/provsync/internal/model"
	"github.com/armaniacs/provsync/internal/secret"
)

// Root はパス解決の基準ディレクトリ群。テストで差し替え可能。
type Root struct {
	ConfigHome string
	StateHome  string
}

// NewRoot は $HOME と XDG 環境変数から Root を組み立てる。
func NewRoot(home string) Root {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		stateHome = filepath.Join(home, ".local", "state")
	}
	return Root{ConfigHome: configHome, StateHome: stateHome}
}

// ResolveRoot は --root の指定を反映した Root を返す。
// override が空なら通常の解決、指定時はその配下に XDG ディレクトリを置く。
func ResolveRoot(home, override string) Root {
	if override == "" {
		return NewRoot(home)
	}
	return Root{
		ConfigHome: filepath.Join(override, ".config"),
		StateHome:  filepath.Join(override, ".local", "state"),
	}
}

// CentralConfigPath はセントラル設定ファイルのパスを返す。
func (r Root) CentralConfigPath() string {
	return filepath.Join(r.ConfigHome, "provsync", "config.json")
}

// StateDir は provsync の状態ディレクトリを返す。
func (r Root) StateDir() string {
	return filepath.Join(r.StateHome, "provsync")
}

// Adapter はツール固有設定とカノニカル表現を変換する。
type Adapter interface {
	// Name はツール識別子を返す。
	Name() string
	// Path はツール設定ファイルの絶対パスを返す。
	Path() string
	// Pull はツール設定を読み、カノニカル表現へ変換する。
	// 2 つ目の戻り値は警告(取り込まなかった秘密など)。言語中立の Message で
	// 返し、描画側が言語を決める。
	Pull() (map[string]model.Provider, []*i18n.Message, error)
	// Push は managed をツール設定へマージしたファイル全文を返す。
	// provider 以外の設定と managed 外の provider は保持する。
	Push(managed map[string]model.Provider) ([]byte, error)
	// Project は managed をこのツールへの push が実際に描画する形へ写す。
	// 意味差分の比較に使い、ツールが描画しないフィールドの誤検知を防ぐ。
	Project(managed map[string]model.Provider) map[string]model.Provider
}

var aliases = map[string]string{"kilo": "kilocode"}

// Names は対応ツールの識別子一覧を返す。
func Names() []string {
	return []string{"kilocode", "opencode"}
}

// Get は名前からアダプタを生成する。別名も受理する。
// toolCandidates はツールごとの設定ファイル候補(ConfigHome からの相対パス、優先順)。
// オフィシャルの読み順に準拠する:
//   - kilocode: kilo.jsonc を正とし、kilo.json も読む
//     (https://kilo.ai/docs/getting-started/settings は kilo.jsonc を正とし、
//     同位置の kilo.json も deep-merge 対象としている)。
//   - opencode: opencode.jsonc → opencode.json → config.json の順に読む
//     (opencode の globalConfigFile と同じ優先順。両拡張子とも JSONC として解釈する)。
//
// 複数候補が存在するとき provsync が管理するのは先頭の 1 ファイルだけである。
// ツール本体のような deep-merge は行わない(単一ファイル管理の設計)。
var toolCandidates = map[string][]string{
	"kilocode": {"kilo/kilo.jsonc", "kilo/kilo.json", "kilo/config.json"},
	"opencode": {"opencode/opencode.jsonc", "opencode/opencode.json", "opencode/config.json"},
}

// resolveToolPath は存在する最初の候補を返す。どれも無いときは先頭候補を返す
// (作成時の置き場所と、エラーメッセージの表示用)。
// 判定は Lstat で行う。リンク切れのシンボリックリンクは候補として採用し、
// 後段の checkReadable / resolveWritePath が正確なリンクエラーを出す。
// Stat でリンク先まで辿ると、壊れリンクが黙って読み飛ばされてしまう。
func resolveToolPath(root Root, tool string) string {
	for _, rel := range toolCandidates[tool] {
		p := filepath.Join(root.ConfigHome, rel)
		if _, err := os.Lstat(p); err == nil {
			return p
		}
	}
	return filepath.Join(root.ConfigHome, toolCandidates[tool][0])
}

func Get(name string, root Root) (Adapter, error) {
	canonical := name
	if a, ok := aliases[name]; ok {
		canonical = a
	}
	switch canonical {
	case "kilocode":
		return &kilocode{path: resolveToolPath(root, "kilocode")}, nil
	case "opencode":
		return &opencode{path: resolveToolPath(root, "opencode")}, nil
	default:
		return nil, i18n.New("err.adapter.unknownTool", name, joinNames())
	}
}

func joinNames() string {
	names := Names()
	sort.Strings(names)
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}

// secretLike は秘密情報らしいキー名かを判定する(大文字小文字は区別しない)。
// 判定は出力層のマスクと共有するため、secret パッケージへ委譲する。
func secretLike(key string) bool {
	return secret.IsKey(key)
}

// decodeProvider はツールの provider エントリをカノニカル表現へ変換する。
// tool は Extras の名前空間に使う。戻り値の 2 番目は警告。
// 秘密情報らしいキーは値を読み込まず警告して取り込まない。
// warnings は言語中立の Message で返す。
func decodeProvider(tool string, entry map[string]any) (model.Provider, []*i18n.Message) {
	var warnings []*i18n.Message
	secretWarning := func(key string) {
		warnings = append(warnings, i18n.New("warn.secret.field", key))
	}
	p := model.Provider{}
	if v, ok := entry["name"].(string); ok {
		p.Name = v
	}
	if v, ok := entry["npm"].(string); ok {
		p.NPM = v
	}
	if opts, ok := entry["options"].(map[string]any); ok {
		if v, ok := opts["baseURL"].(string); ok {
			p.BaseURL = v
		}
		if v, ok := opts["apiKeyEnv"].(string); ok {
			p.APIKeyEnv = v
		}
	}
	if m, ok := entry["models"].(map[string]any); ok {
		p.Models = m
	}

	extras := map[string]any{}
	for k, v := range entry {
		switch k {
		case "name", "npm", "options", "models":
		default:
			if secretLike(k) {
				secretWarning(k)
				continue
			}
			extras[k] = v
		}
	}
	if opts, ok := entry["options"].(map[string]any); ok {
		leftover := map[string]any{}
		for k, v := range opts {
			if k == "baseURL" || k == "apiKeyEnv" {
				continue
			}
			if secretLike(k) {
				secretWarning("options." + k)
				continue
			}
			leftover[k] = v
		}
		if len(leftover) > 0 {
			extras["options"] = leftover
		}
	}
	p.SetExtra(tool, extras)
	return p, warnings
}

// encodeProvider はカノニカル表現をツールの provider エントリへ変換する。
// supportsAPIKeyEnv が false のツールには apiKeyEnv を書き出さない。
func encodeProvider(tool string, p model.Provider, supportsAPIKeyEnv bool) map[string]any {
	entry := cloneMap(p.Extra(tool))
	if p.Name != "" {
		entry["name"] = p.Name
	}
	if p.NPM != "" {
		entry["npm"] = p.NPM
	}
	var opts map[string]any
	if v, ok := entry["options"].(map[string]any); ok {
		opts = cloneMap(v)
	} else {
		opts = map[string]any{}
	}
	if p.BaseURL != "" {
		opts["baseURL"] = p.BaseURL
	}
	if supportsAPIKeyEnv && p.APIKeyEnv != "" {
		opts["apiKeyEnv"] = p.APIKeyEnv
	}
	if len(opts) > 0 {
		entry["options"] = opts
	} else {
		delete(entry, "options")
	}
	if p.Models != nil {
		entry["models"] = p.Models
	}
	return entry
}

func cloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// extractProviders は doc の provider セクションを map として返す。
func extractProviders(doc map[string]any) map[string]any {
	if v, ok := doc["provider"].(map[string]any); ok {
		return v
	}
	return map[string]any{}
}

// decodeProviders は provider セクション全体を変換する。
func decodeProviders(tool string, section map[string]any) (map[string]model.Provider, []*i18n.Message) {
	out := make(map[string]model.Provider, len(section))
	var warnings []*i18n.Message
	for key, raw := range section {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		p, w := decodeProvider(tool, entry)
		out[key] = p
		warnings = append(warnings, w...)
	}
	return out, warnings
}

// fileExists はパスの存在有無を返す。
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// decodeDocument は生バイトを JSON/JSONC から map へ変換する。
func decodeDocument(path string, raw []byte, stripComments bool) (map[string]any, error) {
	if stripComments {
		raw = jsonc.StripJSONC(raw)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		kind := "JSON"
		if stripComments {
			kind = "JSONC"
		}
		return nil, i18n.Wrap(err, "err.config.invalid", kind, path)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc, nil
}

// readDocument はツール設定ファイルを読んで map へ変換する。
func readDocument(path string, stripComments bool) (map[string]any, error) {
	if !fileExists(path) {
		return nil, i18n.New("err.config.missing", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, i18n.Wrap(err, "err.config.read")
	}
	return decodeDocument(path, raw, stripComments)
}

// pullDocument はツール設定をカノニカル provider 集合へ変換する。
func pullDocument(tool, path string, stripComments bool) (map[string]model.Provider, []*i18n.Message, error) {
	doc, err := readDocument(path, stripComments)
	if err != nil {
		return nil, nil, err
	}
	providers, warnings := decodeProviders(tool, extractProviders(doc))
	return providers, warnings, nil
}

// pushDocument は managed をツール設定へマージしたファイル全文を返す。
func pushDocument(tool, path string, managed map[string]model.Provider, stripComments, supportsAPIKeyEnv bool) ([]byte, error) {
	doc, err := readDocument(path, stripComments)
	if err != nil {
		return nil, err
	}
	section := extractProviders(doc)
	for key, p := range managed {
		entry := encodeProvider(tool, p, supportsAPIKeyEnv)
		if existing, ok := section[key].(map[string]any); ok {
			carryOverSecrets(existing, entry)
		}
		section[key] = entry
	}
	doc["provider"] = section
	data, err := fsutil.MarshalIndentSorted(doc)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// carryOverSecrets は既存エントリ内の秘密情報らしいフィールドを書き戻し先へ
// そのまま運ぶ。秘密は中央経由で仲介しないため、同一ツールのファイル内で保持する。
func carryOverSecrets(existing, entry map[string]any) {
	for k, v := range existing {
		if secretLike(k) {
			entry[k] = v
		}
	}
	opts, ok := existing["options"].(map[string]any)
	if !ok {
		return
	}
	var dst map[string]any
	if v, ok := entry["options"].(map[string]any); ok {
		dst = v
	} else {
		dst = map[string]any{}
		entry["options"] = dst
	}
	for k, v := range opts {
		if secretLike(k) {
			dst[k] = v
		}
	}
}

// projectProviders は managed をツールへの push が描画する形へ写す。
// 意味差分の比較に使い、ツールが描画しないフィールド(apiKeyEnv 等)の誤検知を防ぐ。
// ファイル内の秘密は比較対象外のため写し込まない。
func projectProviders(tool string, managed map[string]model.Provider, supportsAPIKeyEnv bool) map[string]model.Provider {
	section := make(map[string]any, len(managed))
	for key, p := range managed {
		section[key] = encodeProvider(tool, p, supportsAPIKeyEnv)
	}
	out, _ := decodeProviders(tool, section)
	return out
}
