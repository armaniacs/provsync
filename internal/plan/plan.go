// Package plan は変更計画(Plan)と、その意味差分を扱う。
//
// Plan は「どのファイルの Before/After をどう変えるか」の単一情報源であり、
// プレビュー・diff・書き込み・バックアップがすべて同じ Plan を参照する。
package plan

import (
	"bytes"
	"reflect"
	"sort"

	"github.com/armaniacs/provsync/internal/model"
)

// ProviderChange は 1 provider の意味的な変更を表す。
type ProviderChange struct {
	Key    string   // provider 名
	Op     string   // added | updated | unchanged
	Fields []string // 変更されたカノニカルフィールド
}

// FileChange は 1 ファイルの変更計画。
type FileChange struct {
	Tool     string // "central" | ツール名
	Path     string
	Before   []byte
	After    []byte
	Semantic []ProviderChange
}

// Plan は適用される変更の集合。
type Plan struct {
	Changes []FileChange
}

// Changed は実際にファイル内容が変わる変更が含まれるかを返す。
func (p Plan) Changed() bool {
	for _, c := range p.Changes {
		if !bytes.Equal(c.Before, c.After) {
			return true
		}
	}
	return false
}

// ProvidersDiff は desired を正として current との意味差分を返す。
// desired にしか無いキーは added、差分があるものは updated、
// 一致するものは unchanged とする(removed は managed 上書き方針では発生しない)。
func ProvidersDiff(desired, current map[string]model.Provider) []ProviderChange {
	keys := make([]string, 0, len(desired))
	for k := range desired {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]ProviderChange, 0, len(keys))
	for _, k := range keys {
		d := desired[k]
		c, ok := current[k]
		if !ok {
			out = append(out, ProviderChange{Key: k, Op: "added"})
			continue
		}
		fields := ChangedFields(d, c)
		if len(fields) == 0 {
			out = append(out, ProviderChange{Key: k, Op: "unchanged"})
			continue
		}
		out = append(out, ProviderChange{Key: k, Op: "updated", Fields: fields})
	}
	return out
}

// ChangedFields は 2 つの provider で異なるカノニカルフィールド名を返す
// (ツール別 extras も比較対象に含む)。ツールが描画しないフィールドを含める
// 場合は、比較前に adapter の Project でツール可視の形へ写すこと。
func ChangedFields(a, b model.Provider) []string {
	var fields []string
	if a.Name != b.Name {
		fields = append(fields, "name")
	}
	if a.NPM != b.NPM {
		fields = append(fields, "npm")
	}
	if a.BaseURL != b.BaseURL {
		fields = append(fields, "baseURL")
	}
	if a.APIKeyEnv != b.APIKeyEnv {
		fields = append(fields, "apiKeyEnv")
	}
	if !modelsEqual(a.Models, b.Models) {
		fields = append(fields, "models")
	}
	if !reflect.DeepEqual(a.Extras, b.Extras) {
		fields = append(fields, "x")
	}
	return fields
}

// modelsEqual は models の等価性を判定する。nil と空 map は等価として扱う。
// 中央設定への保存で空の models は省略されるため、省略後(nil)と
// 空オブジェクト({})をそのまま比較すると、ファイルが変わらない偽差分が発生する。
func modelsEqual(a, b map[string]any) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}
