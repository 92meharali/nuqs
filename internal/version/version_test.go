// Tests version comparison for every scheme: known orderings, bad input and a fuzz test.

package version

import (
	"strings"
	"testing"
)

// ordered lists versions from oldest to newest. Every pair is checked both
// ways, which also catches orderings that aren't consistent.
var ordered = map[Scheme][]string{
	Semver: {
		"0.9.0", "1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta",
		"1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "1.2.0", "1.9.0", "1.10.0", "2.0.0",
	},
	Debian: {
		"0.9", "1.0~rc1", "1.0~rc2", "1.0", "1.0-1", "1.0-1ubuntu1", "1.0-1ubuntu1.2", "1.0-1ubuntu1.10",
		"1.0a", "1.0+dfsg", "1.0.1", "1.10", "9.6p1-3ubuntu13", "9.6p1-3ubuntu13.5", "1:0.1", "2:0.1",
	},
	RPM: {
		"0.9", "1.0~rc1", "1.0~rc2", "1.0", "1.0^git1", "1.0^git2", "1.0a", "1.0.1",
		"1.01.1", "1.10", "5.5p1", "5.5p10", "1:0.1",
	},
	OpenSSH: {
		"5.3", "5.3p1", "7.4p1", "7.9p1", "8.0p1", "9.6", "9.6p1", "9.7p1", "9.10p1", "10.0p1",
	},
}

func TestOrdered(t *testing.T) {
	for s, list := range ordered {
		for i := range list {
			for j := range list {
				want := sign(i - j)
				got, err := Compare(s, list[i], list[j])
				if err != nil {
					t.Fatalf("%v: Compare(%q, %q): %v", s, list[i], list[j], err)
				}
				if got != want {
					t.Errorf("%v: Compare(%q, %q) = %d, want %d", s, list[i], list[j], got, want)
				}
			}
		}
	}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		s    Scheme
		a, b string
		want int
	}{
		// Semver
		{Semver, "v1.2.3", "1.2.3", 0},
		{Semver, "1.2.3+build.5", "1.2.3", 0},
		{Semver, "1.2", "1.2.0", 0},
		{Semver, "1", "1.0.0", 0},
		{Semver, "1.0.0-rc.1+build", "1.0.0-rc.1", 0},
		{Semver, "99999999999999999999.0.0", "1.0.0", 1}, // too big for int, still fine
		// Debian
		{Debian, "1.0", "1.0-0", 0}, // no revision is the same as "0", like dpkg
		{Debian, "1:1.0", "2.0", 1}, // epoch wins
		{Debian, "0:1.0", "1.0", 0},
		{Debian, "1.0~", "1.0", -1},
		{Debian, "1.0~~", "1.0~", -1},
		{Debian, "1.001", "1.1", 0},
		{Debian, "2.30-0ubuntu2", "2.30-0ubuntu10", -1},
		{Debian, "1.2-3-4", "1.2-3-5", -1}, // the last dash starts the revision
		// RPM
		{RPM, "1.0", "1.0-5", 0}, // no release in the advisory matches any release
		{RPM, "1.0-1", "1.0-2", -1},
		{RPM, "1.0-1.el9", "1.0-1.el9_2", -1},
		{RPM, "1.0", "1_0", 0},     // separators don't matter
		{RPM, "6.0.rc1", "6.0", 1}, // rpm's famous surprise, "~" exists for this
		{RPM, "1.0aa", "1.0a", 1},
		{RPM, "a", "1", -1}, // letters are older than numbers
		{RPM, "10xyz", "10.1xyz", -1},
		{RPM, "1.0~rc1^git1", "1.0~rc1", 1},
		{RPM, "1.0^20160101", "1.0.1", -1},
		{RPM, "1.0.", "1.0", 0},
		// OpenSSH
		{OpenSSH, "OpenSSH_9.6p1", "9.6p1", 0},
		{OpenSSH, "9.6p1 Ubuntu 3ubuntu13.5", "9.6p1", 0},
		{OpenSSH, "OpenSSH_8.9p1-hpn", "8.9p1", 0},
		{OpenSSH, "9.6p1", "9.8p1", -1},
	}
	for _, c := range cases {
		got, err := Compare(c.s, c.a, c.b)
		if err != nil {
			t.Errorf("%v: Compare(%q, %q): %v", c.s, c.a, c.b, err)
			continue
		}
		if got != c.want {
			t.Errorf("%v: Compare(%q, %q) = %d, want %d", c.s, c.a, c.b, got, c.want)
		}
	}
}

func TestBadVersions(t *testing.T) {
	bad := map[Scheme][]string{
		Semver:  {"", "abc", "1.2.3.4", "1.x", "1.0.0-", "1.0.0-rc..1", "1..2", "v"},
		Debian:  {"", " ", "a:1.0", ":1.0", "-1:1.0", "+1:1.0", "-1", "1:-1"},
		RPM:     {"", "x:1.0", "-1", "1:"},
		OpenSSH: {"", "9", "9.", ".6", "9.6pX", "abc", "OpenSSH_for_Windows_8.1"},
	}
	for s, list := range bad {
		for _, v := range list {
			if _, err := Compare(s, v, "1.0"); err == nil {
				t.Errorf("%v: Compare(%q, ...) should fail", s, v)
			}
			if _, err := Compare(s, "1.0", v); err == nil {
				t.Errorf("%v: Compare(..., %q) should fail", s, v)
			}
		}
	}
}

func TestErrorNamesScheme(t *testing.T) {
	_, err := Compare(Debian, "", "1.0")
	if err == nil || !strings.HasPrefix(err.Error(), "debian: ") {
		t.Errorf("error should start with the scheme, got %v", err)
	}
}

func TestUnknownScheme(t *testing.T) {
	if _, err := Compare(Scheme(99), "1", "1"); err == nil {
		t.Error("unknown scheme should fail")
	}
}

func TestSchemeString(t *testing.T) {
	cases := map[Scheme]string{Semver: "semver", Debian: "debian", RPM: "rpm", OpenSSH: "openssh", Scheme(9): "scheme(9)"}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("Scheme(%d).String() = %q, want %q", int(s), got, want)
		}
	}
}

// FuzzCompare checks that every scheme is consistent: a version equals itself,
// and swapping a and b flips the answer. It must never panic.
func FuzzCompare(f *testing.F) {
	for _, list := range ordered {
		for i := 1; i < len(list); i++ {
			f.Add(list[i-1], list[i])
		}
	}
	f.Add("1.0~", "1.0^")
	f.Add("1:", ":1")
	f.Add("~~", "~")
	f.Fuzz(func(t *testing.T, a, b string) {
		for _, s := range []Scheme{Semver, Debian, RPM, OpenSSH} {
			ab, errAB := Compare(s, a, b)
			ba, errBA := Compare(s, b, a)
			if (errAB == nil) != (errBA == nil) {
				t.Fatalf("%v: %q vs %q fails only one way: %v / %v", s, a, b, errAB, errBA)
			}
			if errAB == nil && ab != -ba {
				t.Fatalf("%v: Compare(%q, %q) = %d but Compare(%q, %q) = %d", s, a, b, ab, b, a, ba)
			}
			if self, err := Compare(s, a, a); err == nil && self != 0 {
				t.Fatalf("%v: Compare(%q, %q) = %d, want 0", s, a, a, self)
			}
		}
	})
}
