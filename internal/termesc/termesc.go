// Package termesc strips terminal control sequences from untrusted text.
package termesc

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
		switch {
		case c == 0x1b:
			n, keep := consumeESC(s, i, allowSGR)
			if keep {
				out = append(out, s[i:i+n]...)
			}
			i += n
		case c == 0x9b: // 8-bit CSI
			n, keep := consumeCSI(s, i+1, allowSGR)
			if keep {
				out = append(out, 0x1b, '[')
				out = append(out, s[i+1:i+1+n]...)
			}
			i += 1 + n
		case c == 0x9d, c == 0x90, c == 0x98, c == 0x9e, c == 0x9f:
			i += consumeStringSeq(s, i+1)
		case c < 0x20:
			if c == '\t' || c == '\n' {
				out = append(out, c)
			}
			i++
		case c == 0x7f:
			i++
		case c == 0xc2 && i+1 < len(s) && s[i+1] >= 0x80 && s[i+1] <= 0x9f:
			// UTF-8 C1 controls U+0080..U+009F
			i += 2
		default:
			if c >= 0x80 && c <= 0x9f {
				i++
				continue
			}
			out = append(out, c)
			i++
		}
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
	case '\\': // ST on its own
		return 2, false
	default:
		// Two-byte ESC sequences (charset, etc.)
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
		if c == 0x9c { // 8-bit ST
			return j - i + 1
		}
		j++
	}
	return len(s) - i
}
