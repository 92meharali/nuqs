// Package target checks what the user asked us to scan before it ever reaches Nmap.
//
// We accept single IPs, CIDR blocks ("10.0.0.0/24"), last-octet ranges
// ("10.0.0.1-50") and hostnames. Anything else is rejected, which also stops
// tricks like passing "-oN file" as a target.
package target

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"net/netip"
	"strconv"
	"strings"
)

// DefaultLimit is how many addresses one scan may cover unless the user forces it.
const DefaultLimit = 1024

// ErrTooMany is returned when the targets add up to more addresses than allowed.
var ErrTooMany = errors.New("too many targets")

// Parse validates every spec and returns them cleaned up and de-duplicated,
// along with how many addresses they cover. A hostname counts as one address.
// If limit is 0 or less there is no limit.
func Parse(specs []string, limit int64) ([]string, int64, error) {
	var out []string
	seen := map[string]bool{}
	total := big.NewInt(0)

	for _, raw := range specs {
		for _, s := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' }) {
			clean, n, err := parseOne(s)
			if err != nil {
				return nil, 0, err
			}
			if seen[clean] {
				continue
			}
			seen[clean] = true
			out = append(out, clean)
			total.Add(total, n)
		}
	}
	if len(out) == 0 {
		return nil, 0, errors.New("no targets given")
	}
	if limit > 0 && total.Cmp(big.NewInt(limit)) > 0 {
		return nil, 0, fmt.Errorf("%w: %s addresses, the limit is %d (use --force to go past it)", ErrTooMany, total.String(), limit)
	}
	if !total.IsInt64() {
		return out, math.MaxInt64, nil // only possible with huge IPv6 blocks and no limit
	}
	return out, total.Int64(), nil
}

// parseOne validates one spec and returns its clean form and address count.
func parseOne(s string) (string, *big.Int, error) {
	one := big.NewInt(1)
	if s == "" {
		return "", nil, errors.New("empty target")
	}
	if strings.HasPrefix(s, "-") {
		return "", nil, fmt.Errorf("invalid target %q: targets can't start with '-'", s)
	}

	if addr, err := netip.ParseAddr(s); err == nil {
		if addr.Zone() != "" {
			return "", nil, fmt.Errorf("invalid target %q: zoned IPv6 addresses aren't supported", s)
		}
		return addr.String(), one, nil
	}

	if strings.Contains(s, "/") {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			return "", nil, fmt.Errorf("invalid CIDR %q", s)
		}
		p = p.Masked()
		hostBits := p.Addr().BitLen() - p.Bits()
		return p.String(), new(big.Int).Lsh(one, uint(hostBits)), nil
	}

	if i := strings.LastIndex(s, "-"); i > 0 {
		return parseRange(s, i)
	}

	if err := checkHostname(s); err != nil {
		return "", nil, err
	}
	return strings.ToLower(strings.TrimSuffix(s, ".")), one, nil
}

// parseRange handles "10.0.0.1-50", where only the last octet changes.
func parseRange(s string, dash int) (string, *big.Int, error) {
	start, err := netip.ParseAddr(s[:dash])
	if err != nil || !start.Is4() {
		// Not an IP range, so it may be a hostname with a dash in it.
		if herr := checkHostname(s); herr == nil {
			return strings.ToLower(strings.TrimSuffix(s, ".")), big.NewInt(1), nil
		}
		return "", nil, fmt.Errorf("invalid target %q", s)
	}
	end, err := strconv.Atoi(s[dash+1:])
	first := int(start.As4()[3])
	if err != nil || end < first || end > 255 {
		return "", nil, fmt.Errorf("invalid range %q: the end must be a number from %d to 255", s, first)
	}
	return fmt.Sprintf("%s-%d", start, end), big.NewInt(int64(end - first + 1)), nil
}

// checkHostname applies the normal DNS rules: labels of letters, digits and
// dashes, no dash at either end of a label, 253 characters at most.
func checkHostname(s string) error {
	h := strings.TrimSuffix(s, ".")
	if h == "" || len(h) > 253 {
		return fmt.Errorf("invalid hostname %q", s)
	}
	for _, label := range strings.Split(h, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return fmt.Errorf("invalid hostname %q", s)
		}
		for _, r := range label {
			ok := r == '-' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
			if !ok {
				return fmt.Errorf("invalid hostname %q", s)
			}
		}
	}
	return nil
}
