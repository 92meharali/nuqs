// Package checks lists every check that ships with Nuqs.
//
// To add a check, create a folder next to the others, implement check.Check,
// write its tests and add it to All below. The tests in this package make sure
// every check in the list is valid and has its own tests.
package checks

import (
	"github.com/PremadeS/nuqs/checks/ftpanon"
	"github.com/PremadeS/nuqs/checks/httpheaders"
	"github.com/PremadeS/nuqs/checks/telnet"
	"github.com/PremadeS/nuqs/internal/check"
)

// All returns a fresh list of every built-in check.
func All() []check.Check {
	return []check.Check{
		ftpanon.Check{},
		httpheaders.Check{},
		telnet.Check{},
	}
}
