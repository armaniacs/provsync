package cli

import "fmt"

// UsageError はコマンドの使い方の誤りを表す。main が終了コード 2 に対応づける。
type UsageError struct{ Msg string }

func (e *UsageError) Error() string { return e.Msg }

func usageErr(format string, a ...any) error {
	return &UsageError{Msg: fmt.Sprintf(format, a...)}
}
