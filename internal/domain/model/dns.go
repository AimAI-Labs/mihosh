package model

import (
	"fmt"
	"net/netip"
	"strings"
)

// DNSQuestion represents a question entry returned by Mihomo /dns/query.
type DNSQuestion struct {
	Name   string `json:"Name"`
	Qtype  uint16 `json:"Qtype"`
	Qclass uint16 `json:"Qclass"`
}

// DNSAnswer represents a single DNS resource record from Mihomo /dns/query.
type DNSAnswer struct {
	Name string `json:"name"`
	Type uint16 `json:"type"`
	TTL  uint32 `json:"TTL"`
	Data string `json:"data"`
}

// DNSQueryResponse represents the DoH-style JSON response from Mihomo /dns/query.
type DNSQueryResponse struct {
	Status     int           `json:"Status"`
	TC         bool          `json:"TC,omitempty"`
	RD         bool          `json:"RD,omitempty"`
	RA         bool          `json:"RA,omitempty"`
	AD         bool          `json:"AD,omitempty"`
	CD         bool          `json:"CD,omitempty"`
	Question   []DNSQuestion `json:"Question,omitempty"`
	Answer     []DNSAnswer   `json:"Answer,omitempty"`
	Authority  []DNSAnswer   `json:"Authority,omitempty"`
	Additional []DNSAnswer   `json:"Additional,omitempty"`
}

var (
	fakeIPv4Prefix = netip.MustParsePrefix("198.18.0.0/15")
	fakeIPv6Prefix = netip.MustParsePrefix("fc00::/18")
)

// DNSRecordTypes lists commonly queried DNS record types in the TUI selector.
var DNSRecordTypes = []string{"A", "AAAA", "CNAME", "TXT", "MX", "NS", "HTTPS", "SRV", "PTR", "ANY"}

// DNSStatusName converts a numeric DNS RCODE into its standard short name.
func DNSStatusName(status int) string {
	switch status {
	case 0:
		return "NOERROR"
	case 1:
		return "FORMERR"
	case 2:
		return "SERVFAIL"
	case 3:
		return "NXDOMAIN"
	case 4:
		return "NOTIMP"
	case 5:
		return "REFUSED"
	default:
		return fmt.Sprintf("RCODE(%d)", status)
	}
}

// DNSRecordTypeName converts a numeric DNS RR type to its mnemonic name.
func DNSRecordTypeName(qtype uint16) string {
	switch qtype {
	case 1:
		return "A"
	case 2:
		return "NS"
	case 5:
		return "CNAME"
	case 6:
		return "SOA"
	case 12:
		return "PTR"
	case 15:
		return "MX"
	case 16:
		return "TXT"
	case 28:
		return "AAAA"
	case 33:
		return "SRV"
	case 65:
		return "HTTPS"
	case 255:
		return "ANY"
	default:
		return fmt.Sprintf("TYPE%d", qtype)
	}
}

// IsFakeIP reports whether addr lies within Mihomo's standard Fake-IP address pools
// (198.18.0.0/15 for IPv4, fc00::/18 for IPv6).
func IsFakeIP(raw string) bool {
	addr, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	if addr.Is4() {
		return fakeIPv4Prefix.Contains(addr)
	}
	return fakeIPv6Prefix.Contains(addr)
}

// ClassifyDNSIP tags notable IP answers for troubleshooting Fake-IP mapping or DNS pollution.
func ClassifyDNSIP(raw string) string {
	addr, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	addr = addr.Unmap()
	if IsFakeIP(addr.String()) {
		return "fake-ip"
	}
	if addr.IsLoopback() || addr.IsUnspecified() {
		return "loopback"
	}
	if addr.IsPrivate() {
		return "private"
	}
	return ""
}

// HasFakeIP returns true if any answer record resolves to a Fake-IP address.
func (r *DNSQueryResponse) HasFakeIP() bool {
	if r == nil {
		return false
	}
	for _, ans := range r.Answer {
		if IsFakeIP(ans.Data) {
			return true
		}
	}
	return false
}
