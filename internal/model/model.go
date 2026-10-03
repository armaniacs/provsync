// Package model はツール非依存のカノニカル表現を定義する。
package model

// Version はセントラル設定ファイルのスキーマバージョン。
const Version = 1

// Config はセントラル設定ファイルの内容。
type Config struct {
	Version   int                          `json:"version"`
	Providers map[string]Provider          `json:"providers"`
	Aliases   map[string]map[string]string `json:"aliases,omitempty"`
	Routes    map[string][]string          `json:"routes,omitempty"`
}

// NewConfig は空のカノニカル設定を返す。
func NewConfig() *Config {
	return &Config{Version: Version, Providers: map[string]Provider{}}
}

// Provider は正規化された provider エントリ。
//
// 既知フィールドは正規化して保持し、ツール固有の未知フィールドは
// Extras にツール名で名前空間化して保存する。これにより
// pull -> push の往復で情報を失わない。
type Provider struct {
	Name      string                    `json:"name,omitempty"`
	NPM       string                    `json:"npm,omitempty"`
	BaseURL   string                    `json:"baseURL,omitempty"`
	APIKeyEnv string                    `json:"apiKeyEnv,omitempty"`
	Models    map[string]any            `json:"models,omitempty"`
	Extras    map[string]map[string]any `json:"x,omitempty"`
}

// Extra は指定ツール向けの未知フィールド群を返す。
func (p Provider) Extra(tool string) map[string]any {
	if p.Extras == nil {
		return nil
	}
	return p.Extras[tool]
}

// SetExtra は指定ツール向けの未知フィールド群を設定する。空なら何もしない。
func (p *Provider) SetExtra(tool string, m map[string]any) {
	if len(m) == 0 {
		return
	}
	if p.Extras == nil {
		p.Extras = map[string]map[string]any{}
	}
	p.Extras[tool] = m
}
