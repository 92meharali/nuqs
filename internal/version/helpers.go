// Small helpers shared by the schemes.

package version

import (
	"fmt"
	"strconv"
	"strings"
)

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func isAlpha(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func isAlnum(c byte) bool { return isDigit(c) || isAlpha(c) }

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return true
}

// cmpNum compares two digit strings of any length without overflowing.
func cmpNum(a, b string) int {
	a = strings.TrimLeft(a, "0")
	b = strings.TrimLeft(b, "0")
	if len(a) != len(b) {
		return len(a) - len(b)
	}
	return strings.Compare(a, b)
}

// splitEpoch splits "1:2.0" into 1 and "2.0". No epoch means 0.
func splitEpoch(v string) (int, string, error) {
	e, rest, ok := strings.Cut(v, ":")
	if !ok {
		return 0, v, nil
	}
	n, err := strconv.Atoi(e)
	if err != nil || !allDigits(e) {
		return 0, "", fmt.Errorf("bad epoch in %q", v)
	}
	return n, rest, nil
}

// splitLast splits at the last sep, like "1.0-1" into "1.0" and "1".
func splitLast(v string, sep byte) (string, string) {
	if i := strings.LastIndexByte(v, sep); i >= 0 {
		return v[:i], v[i+1:]
	}
	return v, ""
}
