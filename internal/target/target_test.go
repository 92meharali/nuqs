package target

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestParseValid(t *testing.T) {
	cases := []struct {
		in    []string
		want  []string
		count int64
	}{
		{[]string{"10.0.0.1"}, []string{"10.0.0.1"}, 1},
		{[]string{"10.0.0.0/24"}, []string{"10.0.0.0/24"}, 256},
		{[]string{"10.0.0.7/24"}, []string{"10.0.0.0/24"}, 256}, // host bits get masked
		{[]string{"10.0.0.1/32"}, []string{"10.0.0.1/32"}, 1},
		{[]string{"10.0.0.1-20"}, []string{"10.0.0.1-20"}, 20},
		{[]string{"10.0.0.5-5"}, []string{"10.0.0.5-5"}, 1},
		{[]string{"Example.COM"}, []string{"example.com"}, 1},
		{[]string{"example.com."}, []string{"example.com"}, 1},
		{[]string{"my-host"}, []string{"my-host"}, 1},
		{[]string{"router-1.lan"}, []string{"router-1.lan"}, 1},
		{[]string{"localhost"}, []string{"localhost"}, 1},
		{[]string{"::1"}, []string{"::1"}, 1},
		{[]string{"2001:db8::/120"}, []string{"2001:db8::/120"}, 256},
		{[]string{"10.0.0.1,10.0.0.2"}, []string{"10.0.0.1", "10.0.0.2"}, 2},
		{[]string{"10.0.0.1 10.0.0.2\t10.0.0.3\n"}, []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}, 3},
		{[]string{"10.0.0.1", "10.0.0.1"}, []string{"10.0.0.1"}, 1}, // duplicates dropped
	}
	for _, c := range cases {
		got, n, err := Parse(c.in, DefaultLimit)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", c.in, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) || n != c.count {
			t.Errorf("Parse(%q) = %q, %d, want %q, %d", c.in, got, n, c.want, c.count)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	bad := []string{
		"", "-oN", "-iL", "--script=evil", "-",
		"10.0.0.1-300", "10.0.0.20-10", "10.0.0.1-x", "10.0.0.1-",
		"10.0.0.0/33", "10.0.0/8", "::1/129",
		"bad_host", "host..com", ".host", "-host", "host-",
		"fe80::1%en0", "http://example.com", "10.0.0.1;rm", strings.Repeat("a", 64) + ".com",
		strings.Repeat("a.", 130) + "com",
	}
	for _, in := range bad {
		if got, _, err := Parse([]string{in}, DefaultLimit); err == nil {
			t.Errorf("Parse(%q) should fail, got %q", in, got)
		}
	}
}

func TestParseEmpty(t *testing.T) {
	for _, in := range [][]string{nil, {}, {" "}, {","}} {
		if _, _, err := Parse(in, DefaultLimit); err == nil {
			t.Errorf("Parse(%q) should fail", in)
		}
	}
}

func TestParseLimit(t *testing.T) {
	_, _, err := Parse([]string{"10.0.0.0/16"}, DefaultLimit)
	if !errors.Is(err, ErrTooMany) {
		t.Fatalf("expected ErrTooMany, got %v", err)
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error should tell the user about --force: %v", err)
	}
	// Exactly at the limit is fine.
	if _, n, err := Parse([]string{"10.0.0.0/22"}, 1024); err != nil || n != 1024 {
		t.Errorf("a /22 is exactly 1024 addresses, got %d, %v", n, err)
	}
	// The limit counts across all specs.
	if _, _, err := Parse([]string{"10.0.0.0/22", "10.1.0.1"}, 1024); !errors.Is(err, ErrTooMany) {
		t.Errorf("limit should add up across specs, got %v", err)
	}
	// No limit.
	if _, n, err := Parse([]string{"10.0.0.0/8"}, 0); err != nil || n != 1<<24 {
		t.Errorf("no limit: got %d, %v", n, err)
	}
	// Huge IPv6 blocks don't overflow.
	if _, n, err := Parse([]string{"2001:db8::/32"}, 0); err != nil || n != math.MaxInt64 {
		t.Errorf("huge block: got %d, %v", n, err)
	}
}

// FuzzParse makes sure no input can crash Parse, and that anything it accepts
// is safe to hand to Nmap (never starts with '-', never contains spaces).
func FuzzParse(f *testing.F) {
	for _, s := range []string{"10.0.0.1", "10.0.0.0/24", "10.0.0.1-5", "example.com", "-oN", "::1", "a-b.c", "1.2.3.4-"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out, _, err := Parse([]string{s}, 0)
		if err != nil {
			return
		}
		for _, o := range out {
			if o == "" || strings.HasPrefix(o, "-") || strings.ContainsAny(o, " \t\n,;'\"$`|&<>") {
				t.Fatalf("Parse(%q) accepted unsafe target %q", s, o)
			}
		}
	})
}
