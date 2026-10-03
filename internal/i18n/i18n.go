// Package i18n はユーザー向けメッセージの言語判定とカタログ参照を提供する。
// ja が正で、en は mirror。未対応のロケールは ja にフォールバックする。
package i18n

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// 対応する言語の識別子。
const (
	LangJa = "ja"
	LangEn = "en"
)

// Resolve はロケール値 1 件を判定する純粋関数。
// 前方一致で ja* → ja、en* → en。それ以外・空は ja にフォールバックし、エラーにしない。
func Resolve(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	switch {
	case strings.HasPrefix(v, "ja"):
		return LangJa
	case strings.HasPrefix(v, "en"):
		return LangEn
	default:
		return LangJa
	}
}

// ResolvePriority は優先順に並んだロケール値から判定する純粋関数。
// 空文字列は読み飛ばし、すべて空なら ja を返す。
func ResolvePriority(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) == "" {
			continue
		}
		return Resolve(v)
	}
	return LangJa
}

// ResolveFromEnv は言語系環境変数から実行時の言語を判定する。
// 優先順位は PROVSYNC_LANG > LC_ALL > LC_MESSAGES > LANG。
// os.Getenv の読み取りはここに集約し、プロセス入口(main と cli.RunWith)から
// だけ呼ぶ。プロセス生存中は env が変わらないため、入口の複数回呼び出しでも
// 結果は同一になる。
func ResolveFromEnv() string {
	return ResolvePriority(
		os.Getenv("PROVSYNC_LANG"),
		os.Getenv("LC_ALL"),
		os.Getenv("LC_MESSAGES"),
		os.Getenv("LANG"),
	)
}

// T はカタログからテンプレートを取得し fmt.Sprintf で補間した文字列を返す。
// 未登録の ID は id 自体を返す(カタログ完全性テストが実バグを拾う)。
// テンプレートに %w を含めてはならない。%w のラップは呼び出し側の
// fmt.Errorf("…: %w") が担う。
func T(lang, id string, a ...any) string {
	tpl := lookup(lang, id)
	if tpl == "" {
		return id
	}
	if len(a) == 0 {
		return tpl
	}
	return fmt.Sprintf(tpl, a...)
}

func lookup(lang, id string) string {
	if lang == LangEn {
		if v, ok := enCatalog[id]; ok {
			return v
		}
	}
	return jaCatalog[id]
}

// Message は内部パッケージから cli 境界へ運ぶ言語中立のメッセージ値。
// ID と args のみを保持し、言語を知るのは描画側だけである。
type Message struct {
	ID   string
	Args []any
	// Err は下位の原因。nil のときは単独メッセージ。
	Err error
}

// New は原因なしの Message を作る。
func New(id string, args ...any) *Message {
	return &Message{ID: id, Args: args}
}

// Wrap は下位エラーを原因として包む Message を作る。
// fmt.Errorf("…: %w", err) 相当のラップを言語中立のまま行う。
func Wrap(err error, id string, args ...any) *Message {
	return &Message{ID: id, Args: args, Err: err}
}

// Error は ja(正)で描画する。cli 境界を通らない呼び出し元(既存テストの
// 部分文字列アサート、失敗時のフォールバック)が現行どおり日本語で
// 受け取れるようにするため。原因がある場合は fmt.Errorf の %w と同じ
// "本文: 原因" の形で連結する。
func (m *Message) Error() string {
	s := T(LangJa, m.ID, m.Args...)
	if m.Err != nil {
		return s + ": " + m.Err.Error()
	}
	return s
}

// Unwrap は原因を返す。errors.Unwrap / errors.As の対象になる。
func (m *Message) Unwrap() error { return m.Err }

// Localize は err チェーンを辿り、Message ノードは lang のカタログ訳、
// fmt.Errorf("…: %w") 相当のノードは自分の prefix だけ、末端のプレーンな
// エラーは err.Error() のまま ": " で連結する。
// fmt.Errorf の %w 連鎖の描画と同じ形になるため、ja 実行時の出力テキストは
// 移行前と同一になる。下位ノードの文言は各ノードで 1 回だけ載せるため、
// wrapError の Error() をそのまま使うと起きる二重訳を避けている。
func Localize(lang string, err error) string {
	var parts []string
	for err != nil {
		if m, ok := err.(*Message); ok {
			parts = append(parts, T(lang, m.ID, m.Args...))
			err = errors.Unwrap(err)
			continue
		}
		if wrapped := errors.Unwrap(err); wrapped != nil {
			parts = append(parts, strings.TrimSuffix(err.Error(), ": "+wrapped.Error()))
			err = wrapped
			continue
		}
		parts = append(parts, err.Error())
		break
	}
	return strings.Join(parts, ": ")
}
