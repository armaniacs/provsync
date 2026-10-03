package adapter

import (
	"github.com/armaniacs/provsync/internal/i18n"
	"github.com/armaniacs/provsync/internal/model"
)

// opencode は opencode の JSON / JSONC 設定を扱うアダプタ。
// オフィシャルに従い、両拡張子とも JSONC として解釈する。
type opencode struct {
	path string
}

func (a *opencode) Name() string { return "opencode" }

func (a *opencode) Path() string { return a.path }

func (a *opencode) Pull() (map[string]model.Provider, []*i18n.Message, error) {
	return pullDocument(a.Name(), a.path, true)
}

func (a *opencode) Push(managed map[string]model.Provider) ([]byte, error) {
	// opencode は秘密を auth.json で管理するため apiKeyEnv を書き出さない。
	return pushDocument(a.Name(), a.path, managed, true, false)
}

func (a *opencode) Project(managed map[string]model.Provider) map[string]model.Provider {
	// opencode は秘密を auth.json で管理するため apiKeyEnv を描画しない。
	return projectProviders(a.Name(), managed, false)
}
