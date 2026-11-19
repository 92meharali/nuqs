// Semantic versioning, like 1.2.3 and 1.0.0-rc.1.

package version

import (
	"fmt"
	"strings"
)

// compareSemver follows semver.org. "v1.2" is read as "1.2.0", and build
// metadata after "+" never counts.
func compareSemver(a, b string) (int, error) {
	x, xpre, err := parseSemver(a)
	if err != nil {
		return 0, err
	}
	y, ypre, err := parseSemver(b)
	if err != nil {
		return 0, err
	}
	for i := range x {
		if c := cmpNum(x[i], y[i]); c != 0 {
			return c, nil
		}
	}
	return comparePre(xpre, ypre), nil
}

func parseSemver(v string) (nums [3]string, pre []string, err error) {
	s := strings.TrimPrefix(v, "v")
	s, _, _ = strings.Cut(s, "+")
	s, p, hasPre := strings.Cut(s, "-")
	parts := strings.Split(s, ".")
	if len(parts) > 3 {
		return nums, nil, fmt.Errorf("bad version %q", v)
	}
	nums = [3]string{"0", "0", "0"}
	for i, part := range parts {
		if !allDigits(part) {
			return nums, nil, fmt.Errorf("bad version %q", v)
		}
		nums[i] = part
	}
	if hasPre {
		pre = strings.Split(p, ".")
		for _, id := range pre {
			if id == "" {
				return nums, nil, fmt.Errorf("bad pre-release in %q", v)
			}
		}
	}
	return nums, pre, nil
}

// comparePre orders pre-releases. No pre-release is newer than any:
// 1.0.0-rc.1 < 1.0.0.
func comparePre(x, y []string) int {
	switch {
	case x == nil && y == nil:
		return 0
	case x == nil:
		return 1
	case y == nil:
		return -1
	}
	for i := 0; i < len(x) && i < len(y); i++ {
		p, q := x[i], y[i]
		pn, qn := allDigits(p), allDigits(q)
		var c int
		switch {
		case pn && qn:
			c = cmpNum(p, q)
		case pn:
			c = -1 // numbers sort before words
		case qn:
			c = 1
		default:
			c = strings.Compare(p, q)
		}
		if c != 0 {
			return c
		}
	}
	return len(x) - len(y)
}
