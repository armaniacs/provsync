// Package store は中央カノニカル設定ファイルの読み書きを担う。
package store

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/armaniacs/provsync/internal/fsutil"
	"github.com/armaniacs/provsync/internal/model"
)

// Load は中央設定を読み込む。ファイルが無い場合はエラー。
func Load(path string) (*model.Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("中央設定を読めません: %w", err)
	}
	var cfg model.Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("中央設定の JSON が不正です: %w", err)
	}
	if cfg.Version == 0 {
		cfg.Version = model.Version
	}
	if cfg.Providers == nil {
		cfg.Providers = map[string]model.Provider{}
	}
	return &cfg, nil
}

// Marshal はカノニカル設定を整形済み JSON バイト列に変換する。
// ファイルへの書き込みは Plan の適用側(fsutil.WriteFileAtomic)が行う。
func Marshal(cfg *model.Config) ([]byte, error) {
	if cfg.Version == 0 {
		cfg.Version = model.Version
	}
	if cfg.Providers == nil {
		cfg.Providers = map[string]model.Provider{}
	}
	data, err := fsutil.MarshalIndentSorted(cfg)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
