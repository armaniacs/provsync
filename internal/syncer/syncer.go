// Package syncer は provider 集合のマージ・絞り込みを提供する。
package syncer

import (
	"fmt"
	"sort"

	"github.com/armaniacs/provsync/internal/model"
)

// MergeToolProviders は base を土台に、tool からの incoming で上書きした
// 新しい集合を返す。上書きは該当ツールの名前空間(Extras[tool])に限られ、
// 他ツールの名前空間は保持する。incoming に無いキーも保持する(安全側)。
func MergeToolProviders(base, incoming map[string]model.Provider, tool string) map[string]model.Provider {
	out := make(map[string]model.Provider, len(base)+len(incoming))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range incoming {
		if prev, ok := out[k]; ok && prev.Extras != nil {
			merged := v
			merged.Extras = make(map[string]map[string]any, len(prev.Extras))
			for ns, ex := range prev.Extras {
				if ns != tool {
					merged.Extras[ns] = ex
				}
			}
			if v.Extras != nil {
				if ex, ok := v.Extras[tool]; ok {
					merged.Extras[tool] = ex
				}
			}
			if len(merged.Extras) == 0 {
				merged.Extras = nil
			}
			out[k] = merged
			continue
		}
		out[k] = v
	}
	return out
}

// FilterProviders は keys に挙げた provider だけを抽出する。
// keys が空なら全件を返す。keys に存在しない名前があればエラー。
func FilterProviders(providers map[string]model.Provider, keys []string) (map[string]model.Provider, error) {
	if len(keys) == 0 {
		out := make(map[string]model.Provider, len(providers))
		for k, v := range providers {
			out[k] = v
		}
		return out, nil
	}
	out := make(map[string]model.Provider, len(keys))
	var missing []string
	for _, k := range keys {
		v, ok := providers[k]
		if !ok {
			missing = append(missing, k)
			continue
		}
		out[k] = v
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("provider が見つかりません: %v", missing)
	}
	return out, nil
}
