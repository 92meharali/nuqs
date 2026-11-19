// Debian and Ubuntu package versions, compared like dpkg does.

package version

import (
	"errors"
	"fmt"
	"strings"
)

// compareDebian compares [epoch:]upstream[-revision]: epoch first, then
// upstream, then revision. A missing revision is the same as "0", like dpkg.
func compareDebian(a, b string) (int, error) {
	ea, ua, ra, err := parseDebian(a)
	if err != nil {
		return 0, err
	}
	eb, ub, rb, err := parseDebian(b)
	if err != nil {
		return 0, err
	}
	if ea != eb {
		return ea - eb, nil
	}
	if c := dpkgCmp(ua, ub); c != 0 {
		return c, nil
	}
	return dpkgCmp(ra, rb), nil
}

func parseDebian(v string) (epoch int, upstream, revision string, err error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, "", "", errors.New("empty version")
	}
	epoch, rest, err := splitEpoch(v)
	if err != nil {
		return 0, "", "", err
	}
	upstream, revision = splitLast(rest, '-')
	if upstream == "" {
		return 0, "", "", fmt.Errorf("bad version %q", v)
	}
	return epoch, upstream, revision, nil
}

// dpkgOrder is dpkg's sort order for one character: "~" sorts before
// everything, even the end of the string, then letters, then other symbols.
func dpkgOrder(s string, i int) int {
	if i >= len(s) {
		return 0
	}
	c := s[i]
	switch {
	case isDigit(c):
		return 0
	case isAlpha(c):
		return int(c)
	case c == '~':
		return -1
	}
	return int(c) + 256
}

// dpkgCmp is dpkg's verrevcmp: alternate runs of non-digits and digits.
func dpkgCmp(a, b string) int {
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		for i < len(a) && !isDigit(a[i]) || j < len(b) && !isDigit(b[j]) {
			if ac, bc := dpkgOrder(a, i), dpkgOrder(b, j); ac != bc {
				return ac - bc
			}
			i++
			j++
		}
		si := i
		for i < len(a) && isDigit(a[i]) {
			i++
		}
		sj := j
		for j < len(b) && isDigit(b[j]) {
			j++
		}
		if c := cmpNum(a[si:i], b[sj:j]); c != 0 {
			return c
		}
	}
	return 0
}
