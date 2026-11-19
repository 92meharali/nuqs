// Test cases for Debian versions.

package version

func init() {
	all["debian"] = cases{
		ordered: []string{
			"0.9", "1.0~~", "1.0~", "1.0~rc1", "1.0~rc2", "1.0", "1.0-1", "1.0-1ubuntu1", "1.0-1ubuntu1.2",
			"1.0-1ubuntu1.10", "1.0a", "1.0+dfsg", "1.0.1", "1.2-3-4", "1.2-3-5", "1.10",
			"2.30-0ubuntu2", "2.30-0ubuntu10", "9.6p1-3ubuntu13", "9.6p1-3ubuntu13.5", "1:0.1", "2:0.1",
		},
		equal: [][2]string{
			{"1.0", "1.0-0"}, // no revision is the same as "0", like dpkg
			{"0:1.0", "1.0"},
			{"1.001", "1.1"},
		},
		bad: []string{"", " ", "a:1.0", ":1.0", "-1:1.0", "+1:1.0", "-1", "1:-1"},
	}
}
