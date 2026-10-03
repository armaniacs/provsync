package syncer

import (
	"github.com/armaniacs/provsync/internal/i18n"
	"github.com/armaniacs/provsync/internal/model"
)

// ResolveAliases は managed の各 provider の Models のキーを、tool 向けの ID へ
// 置き換えたコピーを返す。aliases のキーと一致するモデル名だけを変換し、
// 一致しないキーは通常のモデル ID として素通しする。対応が定義されていない
// ツール向けのエイリアスは warnings に積み、キーはそのまま残す。
// warnings は言語中立の Message で返し、描画側が言語を決める。
// 入力の managed と Models は破壊しない。
func ResolveAliases(managed map[string]model.Provider, aliases map[string]map[string]string, tool string) (map[string]model.Provider, []*i18n.Message) {
	if len(aliases) == 0 {
		out := make(map[string]model.Provider, len(managed))
		for k, p := range managed {
			out[k] = p
		}
		return out, nil
	}

	var warnings []*i18n.Message
	out := make(map[string]model.Provider, len(managed))
	for key, p := range managed {
		models := make(map[string]any, len(p.Models))
		for mk, mv := range p.Models {
			mapping, isAlias := aliases[mk]
			if !isAlias {
				models[mk] = mv
				continue
			}
			id, ok := mapping[tool]
			if !ok || id == "" {
				warnings = append(warnings, i18n.New("warn.alias.undefined", mk, tool))
				models[mk] = mv
				continue
			}
			models[id] = mv
		}
		p.Models = models
		out[key] = p
	}
	return out, warnings
}
