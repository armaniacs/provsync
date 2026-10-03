package cli

import (
	"fmt"

	"github.com/armaniacs/provsync/internal/i18n"
)

// UsageError はコマンドの使い方の誤りを表す。main が終了コード 2 に対応づける。
type UsageError struct{ Msg string }

func (e *UsageError) Error() string { return e.Msg }

// usageErr は使い方の誤りを UsageError として返す。
// 文言は生成時に実行時言語へ翻訳される(cli が lang を知る境界のため)。
func (o *options) usageErr(id string, a ...any) error {
	return &UsageError{Msg: i18n.T(o.lang, id, a...)}
}

// ExitError は特定の終了コードで終了させるためのエラー。メッセージは出さない。
type ExitError struct{ Code int }

func (e *ExitError) Error() string { return fmt.Sprintf("exit %d", e.Code) }
