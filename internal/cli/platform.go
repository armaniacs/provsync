package cli

import (
	"github.com/armaniacs/provsync/internal/i18n"
)

// Supported は実行 OS が対応環境かを返す。
// 言語中立の Message を返し、描画(main の Localize)が言語を決める。
func Supported(goos string) error {
	if goos == "windows" {
		return i18n.New("err.unsupportedOS")
	}
	return nil
}
