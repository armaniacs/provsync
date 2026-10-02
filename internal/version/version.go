// Package version はバイナリのバージョン文字列を解決する。
package version

import (
	"runtime/debug"
	"strings"
)

// Version はビルド時に -ldflags "-X ...version.Version=1.2.3" で注入される。
var Version = ""

// String は表示用のバージョン文字列を返す。
func String() string {
	info, ok := debug.ReadBuildInfo()
	return resolve(Version, info, ok)
}

// resolve は注入値、BuildInfo のモジュール版、"dev" の順に採用する。
func resolve(injected string, info *debug.BuildInfo, ok bool) string {
	if injected != "" {
		return strings.TrimPrefix(injected, "v")
	}
	if ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return strings.TrimPrefix(info.Main.Version, "v")
	}
	return "dev"
}
