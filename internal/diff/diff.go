// Package diff は unified diff(git diff 風)を生成する。
package diff

import (
	"bytes"
	"fmt"
	"strings"
)

// Unified は before と after の unified diff を返す。
// 内容が同じなら空文字列を返す。context は前後に表示する行数。
func Unified(path string, before, after []byte, context int) string {
	if bytes.Equal(before, after) {
		return ""
	}
	a := splitLines(before)
	b := splitLines(after)
	ops := diffOps(a, b)
	ranges := hunkRanges(ops, context)
	if len(ranges) == 0 {
		return ""
	}
	return render(path, ops, ranges)
}

// op は 1 行の差分操作。kind は ' '(一致) / '-'(削除) / '+'(追加)。
type op struct {
	kind byte
	text string
}

func splitLines(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	lines := strings.SplitAfter(string(b), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func diffOps(a, b []string) []op {
	n, m := len(a), len(b)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	ops := make([]op, 0, n+m)
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			ops = append(ops, op{' ', a[i]})
			i++
			j++
		case dp[i+1][j] >= dp[i][j+1]:
			ops = append(ops, op{'-', a[i]})
			i++
		default:
			ops = append(ops, op{'+', b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		ops = append(ops, op{'-', a[i]})
	}
	for ; j < m; j++ {
		ops = append(ops, op{'+', b[j]})
	}
	return ops
}

func hunkRanges(ops []op, context int) [][2]int {
	var ranges [][2]int
	i := 0
	for i < len(ops) {
		if ops[i].kind == ' ' {
			i++
			continue
		}
		start := i - context
		if start < 0 {
			start = 0
		}
		end := i + 1
		for end < len(ops) {
			if ops[end].kind != ' ' {
				end++
				continue
			}
			k := end
			for k < len(ops) && ops[k].kind == ' ' {
				k++
			}
			if k < len(ops) && k-end <= 2*context {
				end = k
				continue
			}
			break
		}
		end += context
		if end > len(ops) {
			end = len(ops)
		}
		ranges = append(ranges, [2]int{start, end})
		i = end
	}
	return ranges
}

// labelPath は git 風の a/ b/ 接頭辞をパスへ付ける。
// 絶対パスではスラッシュが重ならないように連結する。
func labelPath(prefix, path string) string {
	if strings.HasPrefix(path, "/") {
		return prefix + path
	}
	return prefix + "/" + path
}

func render(path string, ops []op, ranges [][2]int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "--- %s\n", labelPath("a", path))
	fmt.Fprintf(&sb, "+++ %s\n", labelPath("b", path))

	aLine, bLine := 1, 1
	idx := 0
	for _, r := range ranges {
		for idx < r[0] {
			if ops[idx].kind != '+' {
				aLine++
			}
			if ops[idx].kind != '-' {
				bLine++
			}
			idx++
		}
		oldStart, newStart := aLine, bLine
		oldCount, newCount := 0, 0
		var body strings.Builder
		for k := r[0]; k < r[1]; k++ {
			o := ops[k]
			body.WriteByte(o.kind)
			body.WriteString(o.text)
			if !strings.HasSuffix(o.text, "\n") {
				body.WriteByte('\n')
			}
			if o.kind != '+' {
				oldCount++
				aLine++
			}
			if o.kind != '-' {
				newCount++
				bLine++
			}
		}
		if oldCount == 0 {
			oldStart = 0
		}
		if newCount == 0 {
			newStart = 0
		}
		fmt.Fprintf(&sb, "@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount)
		sb.WriteString(body.String())
		idx = r[1]
	}
	return sb.String()
}
