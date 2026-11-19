// OpenSSH versions like 9.6p1, as banners and Nmap show them.

package version

import (
	"fmt"
	"strings"
)

// compareOpenSSH compares major, minor, then the portable release ("p1").
// No "p" counts as p0.
func compareOpenSSH(a, b string) (int, error) {
	x, err := parseOpenSSH(a)
	if err != nil {
		return 0, err
	}
	y, err := parseOpenSSH(b)
	if err != nil {
		return 0, err
	}
	for i := range x {
		if c := cmpNum(x[i], y[i]); c != 0 {
			return c, nil
		}
	}
	return 0, nil
}

// parseOpenSSH reads "9.6p1" into 9, 6 and 1. It accepts "OpenSSH_9.6p1" and
// "9.6p1 Ubuntu 3ubuntu13", and ignores everything after a space or dash.
func parseOpenSSH(v string) ([3]string, error) {
	s := strings.TrimPrefix(strings.TrimSpace(v), "OpenSSH_")
	if i := strings.IndexAny(s, " -"); i >= 0 {
		s = s[:i]
	}
	s, port, hasP := strings.Cut(s, "p")
	major, minor, ok := strings.Cut(s, ".")
	if !ok || !allDigits(major) || !allDigits(minor) || hasP && !allDigits(port) {
		return [3]string{}, fmt.Errorf("bad OpenSSH version %q", v)
	}
	if !hasP {
		port = "0"
	}
	return [3]string{major, minor, port}, nil
}
