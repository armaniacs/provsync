package cli

import "fmt"

// Supported は実行 OS が対応環境かを返す。
func Supported(goos string) error {
	if goos == "windows" {
		return fmt.Errorf("Windows は未対応です(対応: macOS / Linux)")
	}
	return nil
}
