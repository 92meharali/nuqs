// Tests shared by every scheme: ordering, bad input, lookup and a fuzz test.

package version

import (
	"strings"
	"testing"
)

// cases is what each scheme's test file fills in.
type cases struct {
	ordered []string // oldest to newest, every pair is checked both ways
	equal   [][2]string
	bad     []string
}

// all maps a scheme to its cases, from the scheme's own test file.
var all = map[string]cases{}

// testScheme runs the shared checks on one scheme's cases.
func testScheme(t *testing.T, s Scheme) {
	t.Helper()
	c, ok := all[s.Name]
	if !ok {
		t.Fatalf("no test cases for %s, add a %s_test.go", s.Name, s.Name)
	}
	for i := range c.ordered {
		for j := range c.ordered {
			got, err := s.Compare(c.ordered[i], c.ordered[j])
			if err != nil {
				t.Fatalf("Compare(%q, %q): %v", c.ordered[i], c.ordered[j], err)
			}
			if want := sign(i - j); got != want {
				t.Errorf("Compare(%q, %q) = %d, want %d", c.ordered[i], c.ordered[j], got, want)
			}
		}
	}
	for _, p := range c.equal {
		if got, err := s.Compare(p[0], p[1]); err != nil || got != 0 {
			t.Errorf("Compare(%q, %q) = %d, %v, want equal", p[0], p[1], got, err)
		}
	}
	for _, v := range c.bad {
		if _, err := s.Compare(v, c.ordered[0]); err == nil {
			t.Errorf("Compare(%q, ...) should fail", v)
		}
		if _, err := s.Compare(c.ordered[0], v); err == nil {
			t.Errorf("Compare(..., %q) should fail", v)
		}
	}
}

func TestSchemes(t *testing.T) {
	for _, s := range All() {
		t.Run(s.Name, func(t *testing.T) { testScheme(t, s) })
	}
}

func TestLookup(t *testing.T) {
	for _, s := range All() {
		got, ok := Lookup(s.Name)
		if !ok || got.Name != s.Name {
			t.Errorf("Lookup(%q) = %v, %v", s.Name, got.Name, ok)
		}
	}
	if _, ok := Lookup("nope"); ok {
		t.Error("Lookup of an unknown name should fail")
	}
}

func TestErrorNamesScheme(t *testing.T) {
	_, err := Debian.Compare("", "1.0")
	if err == nil || !strings.HasPrefix(err.Error(), "debian: ") {
		t.Errorf("error should start with the scheme name, got %v", err)
	}
}

func TestEmptyScheme(t *testing.T) {
	if _, err := (Scheme{}).Compare("1", "1"); err == nil {
		t.Error("an empty Scheme should fail")
	}
}

// FuzzCompare checks every scheme is consistent: a version equals itself,
// and swapping a and b flips the answer. It must never panic.
func FuzzCompare(f *testing.F) {
	for _, c := range all {
		for i := 1; i < len(c.ordered); i++ {
			f.Add(c.ordered[i-1], c.ordered[i])
		}
	}
	f.Add("1.0~", "1.0^")
	f.Add("1:", ":1")
	f.Fuzz(func(t *testing.T, a, b string) {
		for _, s := range All() {
			ab, errAB := s.Compare(a, b)
			ba, errBA := s.Compare(b, a)
			if (errAB == nil) != (errBA == nil) {
				t.Fatalf("%s: %q vs %q fails only one way: %v / %v", s.Name, a, b, errAB, errBA)
			}
			if errAB == nil && ab != -ba {
				t.Fatalf("%s: Compare(%q, %q) = %d but Compare(%q, %q) = %d", s.Name, a, b, ab, b, a, ba)
			}
			if self, err := s.Compare(a, a); err == nil && self != 0 {
				t.Fatalf("%s: Compare(%q, %q) = %d, want 0", s.Name, a, a, self)
			}
		}
	})
}
