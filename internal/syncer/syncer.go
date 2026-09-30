package syncer

import "fmt"

// Merge copies the provider entries named by keys from source into target,
// overwriting entries with the same key. Provider entries not named in keys
// are preserved as-is. Returns an error if a requested key is absent from
// source.
func Merge(source, target map[string]any, keys []string) (map[string]any, error) {
	srcProviders, err := providerSection(source)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	tgtProviders, err := providerSection(target)
	if err != nil {
		return nil, fmt.Errorf("target: %w", err)
	}
	for _, k := range keys {
		entry, ok := srcProviders[k]
		if !ok {
			return nil, fmt.Errorf("provider %q not found in source", k)
		}
		tgtProviders[k] = entry
	}
	target["provider"] = tgtProviders
	return target, nil
}

func providerSection(cfg map[string]any) (map[string]any, error) {
	v, ok := cfg["provider"]
	if !ok {
		return map[string]any{}, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("provider is not an object")
	}
	return m, nil
}
