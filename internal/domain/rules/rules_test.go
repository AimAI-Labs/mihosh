package rules

import "testing"

func TestCanonicalize(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Domain, example.com, DIRECT", "DOMAIN,example.com,DIRECT"},
		{"ip-cidr, 192.168.0.0/16 , DIRECT , NO-RESOLVE", "IP-CIDR,192.168.0.0/16,DIRECT,no-resolve"},
		{"match, REJECT", "MATCH,REJECT"},
		{"  DOMAIN-SUFFIX , foo.bar , Proxy Group  ", "DOMAIN-SUFFIX,foo.bar,Proxy Group"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := Canonicalize(tc.in); got != tc.want {
			t.Errorf("Canonicalize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseKey(t *testing.T) {
	cases := []struct {
		in      string
		wantKey string
		wantNR  bool
	}{
		{"DOMAIN-SUFFIX,example.com,DIRECT", "DOMAIN-SUFFIX,example.com,DIRECT", false},
		{"ip-cidr, 10.0.0.0/8, DIRECT, NO-RESOLVE", "IP-CIDR,10.0.0.0/8,DIRECT", true},
		{"IP-CIDR6, ::1/128, REJECT, no-resolve", "IP-CIDR6,::1/128,REJECT", true},
		{"match, PROXY", "MATCH,,PROXY", false},
		{"INVALID", "", false},
		{"DOMAIN,only-two", "", false},
	}
	for _, tc := range cases {
		key, nr := ParseKey(tc.in)
		if key != tc.wantKey || nr != tc.wantNR {
			t.Errorf("ParseKey(%q) = (%q, %v), want (%q, %v)", tc.in, key, nr, tc.wantKey, tc.wantNR)
		}
	}
}

func TestBuildLineAndMatchLine(t *testing.T) {
	if got := BuildLine("DOMAIN-SUFFIX", " example.com ", " DIRECT ", true); got != "DOMAIN-SUFFIX,example.com,DIRECT" {
		t.Errorf("BuildLine non-IP with noResolve = %q", got)
	}
	if got := BuildLine("IP-CIDR", "10.0.0.0/24", "DIRECT", true); got != "IP-CIDR,10.0.0.0/24,DIRECT,no-resolve" {
		t.Errorf("BuildLine IP with noResolve = %q", got)
	}
	if got := BuildLine("match", "ignored", "PROXY", true); got != "MATCH,PROXY" {
		t.Errorf("BuildLine MATCH = %q", got)
	}
	if got := BuildMatchLine("GEOIP", "CN", "DIRECT", true); got != "GEOIP,CN,DIRECT,no-resolve" {
		t.Errorf("BuildMatchLine GEOIP with noResolve = %q", got)
	}
	if got := BuildMatchLine("MATCH", "", "REJECT", true); got != "MATCH,REJECT" {
		t.Errorf("BuildMatchLine MATCH = %q", got)
	}
}

func TestNormalizeTypeAndIsIPRuleType(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
		ok   bool
	}{
		{"DomainSuffix", "DOMAIN-SUFFIX", true},
		{"IPCIDR", "IP-CIDR", true},
		{"IPCIDR6", "IP-CIDR6", true},
		{"ip-cidr6", "IP-CIDR6", true},
		{"SrcIPCIDR", "SRC-IP-CIDR", true},
		{"Match", "MATCH", true},
		{"CustomUnknown", "", false},
	} {
		got, ok := NormalizeType(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("NormalizeType(%q) = (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}

	for _, ip := range []string{"IP-CIDR", "ip-cidr6", "SRC-IP-CIDR"} {
		if !IsIPRuleType(ip) {
			t.Errorf("IsIPRuleType(%q) = false, want true", ip)
		}
	}
	for _, nonIP := range []string{"DOMAIN", "GEOIP", "MATCH", ""} {
		if IsIPRuleType(nonIP) {
			t.Errorf("IsIPRuleType(%q) = true, want false", nonIP)
		}
	}
}

func TestNormalizeInsertIndex(t *testing.T) {
	cases := []struct {
		index, length, want int
	}{
		{-5, 3, 0},
		{0, 3, 0},
		{1, 3, 0},
		{2, 3, 1},
		{3, 3, 2},
		{4, 3, 3},
		{99, 3, 3},
		{1, 0, 0},
	}
	for _, tc := range cases {
		if got := NormalizeInsertIndex(tc.index, tc.length); got != tc.want {
			t.Errorf("NormalizeInsertIndex(%d, %d) = %d, want %d", tc.index, tc.length, got, tc.want)
		}
	}
}
