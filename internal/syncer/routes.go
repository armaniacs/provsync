package syncer

import (
	"github.com/armaniacs/provsync/internal/model"
)

// SelectRoute は candidates の先頭から、apiKeyEnv が設定済みの provider キーを
// 返す。apiKeyEnv が空の provider は「環境変数不要」として候補に残す。
// lookup は環境変数の有無を返す(テストで差し替える)。値は読まない。
// 見つからなければ ok=false。
func SelectRoute(candidates []string, providers map[string]model.Provider, lookup func(string) bool) (key string, ok bool) {
	for _, k := range candidates {
		p, inSet := providers[k]
		if !inSet {
			continue
		}
		if p.APIKeyEnv == "" || lookup(p.APIKeyEnv) {
			return k, true
		}
	}
	return "", false
}
