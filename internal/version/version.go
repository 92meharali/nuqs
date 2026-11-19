// Package version compares software versions the way each ecosystem does.
//
// Advisories say things like "fixed in 1:9.6p1-3ubuntu13", and Nmap tells us
// what a host runs. Plain string comparison gets these wrong ("9.10" < "9.9"),
// so each scheme follows the real tool's rules.
//
// Each scheme lives in its own file. To add one, write a compare function in
// a new file, give it a Scheme variable and add it to All.
package version

import "fmt"

// Scheme is one ecosystem's way of ordering versions.
type Scheme struct {
	Name string // like "debian", used in error messages and config files
	cmp  func(a, b string) (int, error)
}

// The supported schemes. Each one is in the file with the same name.
var (
	Semver  = Scheme{"semver", compareSemver}   // 1.2.3-rc.1+build, a leading "v" is fine
	Debian  = Scheme{"debian", compareDebian}   // [epoch:]upstream[-revision], like dpkg
	RPM     = Scheme{"rpm", compareRPM}         // [epoch:]version[-release], like rpm
	OpenSSH = Scheme{"openssh", compareOpenSSH} // 9.6p1, also "OpenSSH_9.6p1 Ubuntu-3"
)

// All returns every scheme.
func All() []Scheme { return []Scheme{Semver, Debian, RPM, OpenSSH} }

// Lookup finds a scheme by its name, like "debian".
func Lookup(name string) (Scheme, bool) {
	for _, s := range All() {
		if s.Name == name {
			return s, true
		}
	}
	return Scheme{}, false
}

// Compare returns -1 if a is older than b, 0 if they're the same version and
// +1 if a is newer. It returns an error if either version isn't valid for the
// scheme, so a typo never quietly counts as "not affected".
func (s Scheme) Compare(a, b string) (int, error) {
	if s.cmp == nil {
		return 0, fmt.Errorf("version: empty scheme")
	}
	c, err := s.cmp(a, b)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", s.Name, err)
	}
	return sign(c), nil
}
