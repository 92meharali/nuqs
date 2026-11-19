// Test cases for OpenSSH versions.

package version

func init() {
	all["openssh"] = cases{
		ordered: []string{"5.3", "5.3p1", "7.4p1", "7.9p1", "8.0p1", "9.6", "9.6p1", "9.7p1", "9.10p1", "10.0p1"},
		equal: [][2]string{
			{"OpenSSH_9.6p1", "9.6p1"},
			{"9.6p1 Ubuntu 3ubuntu13.5", "9.6p1"},
			{"OpenSSH_8.9p1-hpn", "8.9p1"},
		},
		bad: []string{"", "9", "9.", ".6", "9.6pX", "abc", "OpenSSH_for_Windows_8.1"},
	}
}
