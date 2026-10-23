// Package termesc strips terminal control sequences from untrusted text.
package termesc

import "unicode/utf8"

// Plain removes escape sequences and C0/C1 controls except tab and newline.
func Plain(s string) string {
	return sanitize(s, false)
}

// ForTUI keeps SGR color sequences (CSI ... m) used by lipgloss/glamour
// and strips OSC/DCS/APC and other controls that can hijack the terminal.
func ForTUI(s string) string {
	return sanitize(s, true)
}

func sanitize(s string, allowSGR bool) string {
	if s == "" {
		return s
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		c := s[i]
		if c == 0x1b {
			n, keep := consumeESC(s, i, allowSGR)
			if keep {
				out = append(out, s[i:i+n]...)
			}
			i += n
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			i++
			continue
		}
		switch {
		case r == '\t' || r == '\n':
			out = append(out, s[i:i+size]...)
		case r < 0x20, r == 0x7f, r >= 0x80 && r <= 0x9f:
			// drop other C0, DEL, and Unicode C1
		default:
			out = append(out, s[i:i+size]...)
		}
		i += size
	}
	return string(out)
}

func consumeESC(s string, i int, allowSGR bool) (n int, keep bool) {
	if i+1 >= len(s) {
		return 1, false
	}
	switch s[i+1] {
	case '[':
		m, keep := consumeCSI(s, i+2, allowSGR)
		return 2 + m, keep
	case ']':
		return 2 + consumeOSC(s, i+2), false
	case 'P', 'X', '^', '_':
		return 2 + consumeStringSeq(s, i+2), false
	case '\\':
		return 2, false
	default:
		return 2, false
	}
}

func consumeCSI(s string, i int, allowSGR bool) (n int, keep bool) {
	j := i
	for j < len(s) {
		c := s[j]
		j++
		if c >= 0x40 && c <= 0x7e {
			if allowSGR && c == 'm' {
				return j - i, true
			}
			return j - i, false
		}
	}
	return len(s) - i, false
}

func consumeOSC(s string, i int) int {
	j := i
	for j < len(s) {
		c := s[j]
		if c == 0x07 {
			return j - i + 1
		}
		if c == 0x1b && j+1 < len(s) && s[j+1] == '\\' {
			return j - i + 2
		}
		j++
	}
	return len(s) - i
}

func consumeStringSeq(s string, i int) int {
	j := i
	for j < len(s) {
		c := s[j]
		if c == 0x1b && j+1 < len(s) && s[j+1] == '\\' {
			return j - i + 2
		}
		if c == 0x9c {
			return j - i + 1
		}
		j++
	}
	return len(s) - i
}
