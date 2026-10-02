package adapter

import "github.com/armaniacs/provsync/internal/model"

// kilocode は kilocode/kilo の JSONC 設定を扱うアダプタ。
type kilocode struct {
	path string
}

func (a *kilocode) Name() string { return "kilocode" }

func (a *kilocode) Path() string { return a.path }

func (a *kilocode) Pull() (map[string]model.Provider, []string, error) {
	return pullDocument(a.Name(), a.path, true)
}

func (a *kilocode) Push(managed map[string]model.Provider) ([]byte, error) {
	return pushDocument(a.Name(), a.path, managed, true, true)
}

func (a *kilocode) Project(managed map[string]model.Provider) map[string]model.Provider {
	return projectProviders(a.Name(), managed, true)
}
