// Package cli のうち、check の疎通確認を担う。
package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/armaniacs/provsync/internal/model"
	"github.com/armaniacs/provsync/internal/store"
	"github.com/armaniacs/provsync/internal/syncer"
)

// ---- check ----

// checkTimeout は疎通確認 1 回あたりのタイムアウト。テストで上書きする。
var checkTimeout = 5 * time.Second

// cmdCheck は中央設定の各 provider の API 到達可否を確認する。
// 通信するのはこのコマンドだけ。秘密の値は出力しない。
func cmdCheck(o *options) error {
	root, err := o.root()
	if err != nil {
		return err
	}
	cfg, err := store.Load(root.CentralConfigPath())
	if err != nil {
		return fmt.Errorf("中央設定がありません。先に pull / init を実行してください: %w", err)
	}
	keys := make([]string, 0, len(cfg.Providers))
	for k := range cfg.Providers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	filtered, err := syncer.FilterProviders(cfg.Providers, o.keys())
	if err != nil {
		return err
	}
	if o.keys() != nil {
		keys = make([]string, 0, len(filtered))
		for k := range filtered {
			keys = append(keys, k)
		}
		sort.Strings(keys)
	}

	for _, k := range keys {
		p := cfg.Providers[k]
		switch {
		case p.APIKeyEnv == "":
			fmt.Fprintf(o.out, "[スキップ] %s: apiKeyEnv が未設定\n", k)
			continue
		}
		if _, ok := os.LookupEnv(p.APIKeyEnv); !ok {
			// 変数の値は読んでも出力しない。
			fmt.Fprintf(o.out, "[スキップ] %s: 環境変数 %s が未設定\n", k, p.APIKeyEnv)
			continue
		}
		if p.BaseURL == "" {
			fmt.Fprintf(o.out, "[スキップ] %s: baseURL が未設定\n", k)
			continue
		}
		checkProvider(o, k, p)
	}
	return nil
}

// checkProvider は 1 provider に GET <BaseURL>/models を送り結果を分類して出す。
func checkProvider(o *options, key string, p model.Provider) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()

	url := strings.TrimSuffix(p.BaseURL, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		fmt.Fprintf(o.out, "[応答異常] %s: リクエストを構築できません\n", key)
		return
	}
	token := os.Getenv(p.APIKeyEnv)
	req.Header.Set("Authorization", "Bearer "+token)

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// エラー文字列には URL やヘッダが入らないよう、分類済みの短い文言にする。
		fmt.Fprintf(o.out, "[到達不可] %s\n", key)
		return
	}
	defer resp.Body.Close()
	ms := time.Since(start).Milliseconds()
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		fmt.Fprintf(o.out, "[OK] %s (%dms)\n", key, ms)
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		fmt.Fprintf(o.out, "[認証失敗] %s\n", key)
	default:
		fmt.Fprintf(o.out, "[応答異常] %s: HTTP %d\n", key, resp.StatusCode)
	}
}
