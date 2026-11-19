// Package version compares software versions the way each ecosystem does.
//
// Advisories say things like "fixed in 1:9.6p1-3ubuntu13", and Nmap tells us
// what a host runs. Plain string comparison gets these wrong ("9.10" < "9.9"),
// so each scheme here follows the real tool's rules: semver, dpkg for Debian
// and Ubuntu, rpm for Red Hat and friends, and OpenSSH's "9.6p1" style.
package version

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Scheme says which rules to compare with.
type Scheme int

// The supported schemes.
const (
	Semver  Scheme = iota // 1.2.3-rc.1+build, a leading "v" is fine
	Debian                // [epoch:]upstream[-revision], like dpkg
	RPM                   // [epoch:]version[-release], like rpm
	OpenSSH               // 9.6p1, also "OpenSSH_9.6p1 Ubuntu-3"
)

// String returns the scheme's name, for error messages.
func (s Scheme) String() string {
	switch s {
	case Semver:
		return "semver"
	case Debian:
		return "debian"
	case RPM:
		return "rpm"
	case OpenSSH:
		return "openssh"
	}
	return fmt.Sprintf("scheme(%d)", int(s))
}

// Compare returns -1 if a is older than b, 0 if they're the same version and
// +1 if a is newer. It returns an error if either version isn't valid for the
// scheme, so a typo never quietly counts as "not affected".
func Compare(s Scheme, a, b string) (int, error) {
	var cmp func(a, b string) (int, error)
	switch s {
	case Semver:
		cmp = compareSemver
	case Debian:
		cmp = compareDebian
	case RPM:
		cmp = compareRPM
	case OpenSSH:
		cmp = compareOpenSSH
	default:
		return 0, fmt.Errorf("unknown version scheme %v", s)
	}
	c, err := cmp(a, b)
	if err != nil {
		return 0, fmt.Errorf("%v: %w", s, err)
	}
	return sign(c), nil
}

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

// splitEpoch splits "1:2.0" into 1 and "2.0". No epoch means 0.
func splitEpoch(v string) (int, string, error) {
	e, rest, ok := strings.Cut(v, ":")
	if !ok {
		return 0, v, nil
	}
	n, err := strconv.Atoi(e)
	if err != nil || n < 0 || e == "" || e[0] == '+' {
		return 0, "", fmt.Errorf("bad epoch in %q", v)
	}
	return n, rest, nil
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

// --- Semver ---

type semver struct {
	nums [3]string
	pre  []string
}

func parseSemver(v string) (semver, error) {
	var s semver
	orig := v
	v = strings.TrimPrefix(v, "v")
	v, _, _ = strings.Cut(v, "+") // build metadata never counts
	v, pre, hasPre := strings.Cut(v, "-")
	parts := strings.Split(v, ".")
	if len(parts) > 3 {
		return s, fmt.Errorf("bad version %q", orig)
	}
	for i := range s.nums {
		s.nums[i] = "0" // "1.2" means "1.2.0"
	}
	for i, p := range parts {
		if !allDigits(p) {
			return s, fmt.Errorf("bad version %q", orig)
		}
		s.nums[i] = p
	}
	if hasPre {
		s.pre = strings.Split(pre, ".")
		for _, id := range s.pre {
			if id == "" {
				return s, fmt.Errorf("bad pre-release in %q", orig)
			}
		}
	}
	return s, nil
}

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

func compareSemver(a, b string) (int, error) {
	x, err := parseSemver(a)
	if err != nil {
		return 0, err
	}
	y, err := parseSemver(b)
	if err != nil {
		return 0, err
	}
	for i := range x.nums {
		if c := cmpNum(x.nums[i], y.nums[i]); c != 0 {
			return c, nil
		}
	}
	// A pre-release is older than the release itself: 1.0.0-rc.1 < 1.0.0.
	switch {
	case x.pre == nil && y.pre == nil:
		return 0, nil
	case x.pre == nil:
		return 1, nil
	case y.pre == nil:
		return -1, nil
	}
	for i := 0; i < len(x.pre) && i < len(y.pre); i++ {
		p, q := x.pre[i], y.pre[i]
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
			return c, nil
		}
	}
	return len(x.pre) - len(y.pre), nil
}

// --- Debian (dpkg) ---

type debian struct {
	epoch              int
	upstream, revision string
}

func parseDebian(v string) (debian, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return debian{}, errors.New("empty version")
	}
	epoch, rest, err := splitEpoch(v)
	if err != nil {
		return debian{}, err
	}
	d := debian{epoch: epoch, upstream: rest}
	if i := strings.LastIndexByte(rest, '-'); i >= 0 {
		d.upstream, d.revision = rest[:i], rest[i+1:]
	}
	if d.upstream == "" {
		return debian{}, fmt.Errorf("bad version %q", v)
	}
	return d, nil
}

func compareDebian(a, b string) (int, error) {
	x, err := parseDebian(a)
	if err != nil {
		return 0, err
	}
	y, err := parseDebian(b)
	if err != nil {
		return 0, err
	}
	if x.epoch != y.epoch {
		return x.epoch - y.epoch, nil
	}
	if c := dpkgCmp(x.upstream, y.upstream); c != 0 {
		return c, nil
	}
	return dpkgCmp(x.revision, y.revision), nil
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

// --- RPM ---

type rpm struct {
	epoch            int
	version, release string
}

func parseRPM(v string) (rpm, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return rpm{}, errors.New("empty version")
	}
	epoch, rest, err := splitEpoch(v)
	if err != nil {
		return rpm{}, err
	}
	r := rpm{epoch: epoch, version: rest}
	if i := strings.LastIndexByte(rest, '-'); i >= 0 {
		r.version, r.release = rest[:i], rest[i+1:]
	}
	if r.version == "" {
		return rpm{}, fmt.Errorf("bad version %q", v)
	}
	return r, nil
}

func compareRPM(a, b string) (int, error) {
	x, err := parseRPM(a)
	if err != nil {
		return 0, err
	}
	y, err := parseRPM(b)
	if err != nil {
		return 0, err
	}
	if x.epoch != y.epoch {
		return x.epoch - y.epoch, nil
	}
	if c := rpmvercmp(x.version, y.version); c != 0 {
		return c, nil
	}
	// Advisories often give only "1.2.3". Then any release of 1.2.3 matches,
	// like dnf does.
	if x.release == "" || y.release == "" {
		return 0, nil
	}
	return rpmvercmp(x.release, y.release), nil
}

func isAlnum(c byte) bool { return isDigit(c) || isAlpha(c) }

// rpmvercmp is rpm's own comparison: runs of digits or letters, separators
// ignored, "~" sorts before everything and "^" after the base version.
func rpmvercmp(a, b string) int {
	if a == b {
		return 0
	}
	i, j := 0, 0
	at := func(s string, k int) byte {
		if k < len(s) {
			return s[k]
		}
		return 0
	}
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

// --- OpenSSH ---

// parseOpenSSH reads "9.6p1" into 9, 6 and 1. It accepts what Nmap and
// banners show, like "OpenSSH_9.6p1" or "9.6p1 Ubuntu 3ubuntu13", and ignores
// everything after the first space or dash.
func parseOpenSSH(v string) ([3]string, error) {
	var n [3]string
	s := strings.TrimSpace(v)
	s = strings.TrimPrefix(s, "OpenSSH_")
	if i := strings.IndexAny(s, " -"); i >= 0 {
		s = s[:i]
	}
	s, port, hasP := strings.Cut(s, "p")
	major, minor, ok := strings.Cut(s, ".")
	if !ok || !allDigits(major) || !allDigits(minor) || hasP && !allDigits(port) {
		return n, fmt.Errorf("bad OpenSSH version %q", v)
	}
	if !hasP {
		port = "0"
	}
	return [3]string{major, minor, port}, nil
}

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
