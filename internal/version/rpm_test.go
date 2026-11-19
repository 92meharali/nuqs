// Test cases for RPM versions.

package version

func init() {
	all["rpm"] = cases{
		ordered: []string{
			"0.9", "1.0~rc1", "1.0~rc1^git1", "1.0~rc2", "1.0", "1.0^git1", "1.0^git2", "1.0^20160101",
			"1.0a", "1.0aa", "1.0.1", "1.01.1", "1.10", "5.5p1", "5.5p10", "6.0.rc1", "10xyz", "10.1xyz", "1:0.1",
		},
		equal: [][2]string{
			{"1.0", "1.0-5"}, // no release in the advisory matches any release
			{"1.0", "1_0"},   // separators don't matter
			{"1.0.", "1.0"},
		},
		bad: []string{"", "x:1.0", "-1", "1:"},
	}
}
