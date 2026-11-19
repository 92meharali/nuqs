// Test cases for semver.

package version

func init() {
	all["semver"] = cases{
		ordered: []string{
			"0.9.0", "1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta", "1.0.0-beta.2",
			"1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "1.2.0", "1.9.0", "1.10.0", "2.0.0",
			"99999999999999999999.0.0", // too big for an int, still fine
		},
		equal: [][2]string{
			{"v1.2.3", "1.2.3"},
			{"1.2.3+build.5", "1.2.3"},
			{"1.2", "1.2.0"},
			{"1", "1.0.0"},
			{"1.0.0-rc.1+build", "1.0.0-rc.1"},
		},
		bad: []string{"", "abc", "1.2.3.4", "1.x", "1.0.0-", "1.0.0-rc..1", "1..2", "v"},
	}
}
