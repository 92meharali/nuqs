// Red Hat, Fedora, SUSE and other RPM package versions, compared like rpm does.

package version

import (
	"errors"
	"fmt"
	"strings"
)

// compareRPM compares [epoch:]version[-release]. If either side has no
// release, any release matches, like dnf does when an advisory says "1.2.3".
func compareRPM(a, b string) (int, error) {
	ea, va, ra, err := parseRPM(a)
	if err != nil {
		return 0, err
	}
	eb, vb, rb, err := parseRPM(b)
	if err != nil {
		return 0, err
	}
	if ea != eb {
		return ea - eb, nil
	}
	if c := rpmvercmp(va, vb); c != 0 || ra == "" || rb == "" {
		return c, nil
	}
	return rpmvercmp(ra, rb), nil
}

func parseRPM(v string) (epoch int, version, release string, err error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, "", "", errors.New("empty version")
	}
	epoch, rest, err := splitEpoch(v)
	if err != nil {
		return 0, "", "", err
	}
	version, release = splitLast(rest, '-')
	if version == "" {
		return 0, "", "", fmt.Errorf("bad version %q", v)
	}
	return epoch, version, release, nil
}

// rpmvercmp is rpm's own comparison: runs of digits or letters, separators
// ignored, "~" sorts before everything and "^" after the base version.
func rpmvercmp(a, b string) int {
	if a == b {
		return 0
	}
	at := func(s string, k int) byte {
		if k < len(s) {
			return s[k]
		}
		return 0
	}
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		for i < len(a) && !isAlnum(a[i]) && a[i] != '~' && a[i] != '^' {
			i++
		}
		for j < len(b) && !isAlnum(b[j]) && b[j] != '~' && b[j] != '^' {
			j++
		}
		ca, cb := at(a, i), at(b, j)
		if ca == '~' || cb == '~' {
			if ca != '~' {
				return 1
			}
			if cb != '~' {
				return -1
			}
			i++
			j++
			continue
		}
		if ca == '^' || cb == '^' {
			switch {
			case i >= len(a):
				return -1
			case j >= len(b):
				return 1
			case ca != '^':
				return 1
			case cb != '^':
				return -1
			}
			i++
			j++
			continue
		}
		if i >= len(a) || j >= len(b) {
			break
		}
		si, sj := i, j
		num := isDigit(a[i])
		match := isAlpha
		if num {
			match = isDigit
		}
		for i < len(a) && match(a[i]) {
			i++
		}
		for j < len(b) && match(b[j]) {
			j++
		}
		if sj == j {
			// Different kinds of run: a number is newer than letters.
			if num {
				return 1
			}
			return -1
		}
		var c int
		if num {
			c = cmpNum(a[si:i], b[sj:j])
		} else {
			c = strings.Compare(a[si:i], b[sj:j])
		}
		if c != 0 {
			return c
		}
	}
	switch {
	case i >= len(a) && j >= len(b):
		return 0
	case i >= len(a):
		return -1
	}
	return 1
}
