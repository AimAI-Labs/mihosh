package sync

import (
	"bytes"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// RedactedPlaceholder replaces sensitive credentials during backup/sync export.
const RedactedPlaceholder = "[REDACTED]"

var (
	uuidPattern        = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	hexTokenPattern    = regexp.MustCompile(`(?i)^[0-9a-f]{16,}$`)
	opaqueTokenPattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{24,}$`)
	dsnKVPassword      = regexp.MustCompile(`(?i)(\bpassword\s*=\s*)('(?:\\'|[^'])*'|"(?:\\"|[^"])*"|\S+)`)
)

var sensitiveQueryKeys = []string{
	"token", "key", "secret", "password", "passwd", "pwd", "auth",
	"sig", "signature", "access_token", "api_key", "apikey",
	"credential", "session", "cookie", "jwt", "uuid", "sub_token", "subscribe_token",
}

var sensitiveYAMLKeys = []string{
	"secret", "password", "passwd", "token", "auth", "authentication",
	"uuid", "private-key", "pre-shared-key", "psk", "username",
	"api-key", "access-key", "secret-key", "client-secret",
	"authorization", "proxy-authorization", "dsn",
}

// ContainsRedacted reports whether s contains the redacted placeholder.
func ContainsRedacted(s string) bool {
	return strings.Contains(s, RedactedPlaceholder)
}

// RedactURL strips credentials from URL userinfo, sensitive query params, and opaque token path segments.
func RedactURL(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !strings.Contains(trimmed, "://") {
		return raw, false
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return raw, false
	}

	redacted := false

	// 1. Userinfo (user:pass@host)
	if u.User != nil {
		u.User = url.User(RedactedPlaceholder)
		redacted = true
	}

	// 2. Path segments looking like UUIDs or long opaque tokens
	if u.Path != "" {
		segments := strings.Split(u.Path, "/")
		for i, seg := range segments {
			if seg == "" || seg == RedactedPlaceholder {
				continue
			}
			if looksLikePathToken(seg) {
				segments[i] = RedactedPlaceholder
				redacted = true
			}
		}
		if redacted {
			u.Path = strings.Join(segments, "/")
			u.RawPath = ""
		}
	}

	// 3. Query parameters with sensitive names or token-like values
	if u.RawQuery != "" {
		q := u.Query()
		queryChanged := false
		for k, vals := range q {
			keySensitive := isSensitiveQueryKey(k)
			for i, v := range vals {
				if v == "" || v == RedactedPlaceholder {
					continue
				}
				if keySensitive || looksLikeSecretValue(v) {
					vals[i] = RedactedPlaceholder
					queryChanged = true
					redacted = true
				}
			}
			q[k] = vals
		}
		if queryChanged {
			u.RawQuery = encodeQueryPreservingPlaceholder(q)
		}
	}

	if !redacted {
		return raw, false
	}
	out := u.String()
	// Restore readable [REDACTED] instead of percent-encoded %5BREDACTED%5D.
	out = strings.ReplaceAll(out, "%5BREDACTED%5D", RedactedPlaceholder)
	out = strings.ReplaceAll(out, "%5bREDACTED%5d", RedactedPlaceholder)
	return out, true
}

func encodeQueryPreservingPlaceholder(q url.Values) string {
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		ek := url.QueryEscape(k)
		for _, v := range q[k] {
			if v == RedactedPlaceholder {
				parts = append(parts, ek+"="+RedactedPlaceholder)
			} else {
				parts = append(parts, ek+"="+url.QueryEscape(v))
			}
		}
	}
	return strings.Join(parts, "&")
}

func isSensitiveQueryKey(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	for _, s := range sensitiveQueryKeys {
		if lower == s || strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

func looksLikePathToken(seg string) bool {
	if uuidPattern.MatchString(seg) || hexTokenPattern.MatchString(seg) {
		return true
	}
	// Avoid redacting normal filenames like subscribe.yaml or clash-config.yml
	if strings.Contains(seg, ".") {
		lower := strings.ToLower(seg)
		for _, ext := range []string{".yaml", ".yml", ".json", ".txt", ".conf", ".list"} {
			if strings.HasSuffix(lower, ext) {
				return false
			}
		}
	}
	return opaqueTokenPattern.MatchString(seg) && hasMixedTokenChars(seg)
}

func looksLikeSecretValue(val string) bool {
	if uuidPattern.MatchString(val) || hexTokenPattern.MatchString(val) {
		return true
	}
	return opaqueTokenPattern.MatchString(val) && hasMixedTokenChars(val)
}

func hasMixedTokenChars(s string) bool {
	hasLetter := false
	hasDigit := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	return hasLetter && hasDigit
}

// RedactDSN masks passwords in PostgreSQL URI or keyword/value DSN strings for safe display.
func RedactDSN(dsn string) string {
	trimmed := strings.TrimSpace(dsn)
	if trimmed == "" {
		return ""
	}
	if strings.Contains(trimmed, "://") {
		if u, err := url.Parse(trimmed); err == nil && u.User != nil {
			username := u.User.Username()
			if _, hasPass := u.User.Password(); hasPass {
				if username != "" {
					u.User = url.UserPassword(username, RedactedPlaceholder)
				} else {
					u.User = url.User(RedactedPlaceholder)
				}
				return strings.ReplaceAll(u.String(), "%5BREDACTED%5D", RedactedPlaceholder)
			}
		}
	}
	return dsnKVPassword.ReplaceAllString(trimmed, "${1}"+RedactedPlaceholder)
}

func isSensitiveYAMLKey(key string) bool {
	norm := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "_", "-"))
	for _, s := range sensitiveYAMLKeys {
		if norm == s || strings.Contains(norm, s) {
			return true
		}
	}
	return false
}

// RedactYAML parses a YAML document and replaces sensitive credential values with RedactedPlaceholder.
func RedactYAML(data []byte) ([]byte, int, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, 0, nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, 0, err
	}
	count := redactYAMLNode(&doc, false)
	if count == 0 {
		return append([]byte(nil), data...), 0, nil
	}
	encoded, err := encodeYAMLNode(&doc)
	if err != nil {
		return nil, 0, err
	}
	return encoded, count, nil
}

func redactYAMLNode(node *yaml.Node, parentSensitive bool) int {
	if node == nil {
		return 0
	}
	count := 0
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			count += redactYAMLNode(child, parentSensitive)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			k := node.Content[i]
			v := node.Content[i+1]
			sensitive := parentSensitive || (k.Kind == yaml.ScalarNode && isSensitiveYAMLKey(k.Value))
			count += redactYAMLNode(v, sensitive)
		}
	case yaml.SequenceNode:
		for _, item := range node.Content {
			count += redactYAMLNode(item, parentSensitive)
		}
	case yaml.ScalarNode:
		if node.ShortTag() == "!!null" || strings.TrimSpace(node.Value) == "" || node.Value == RedactedPlaceholder {
			return 0
		}
		if parentSensitive {
			node.Tag = "!!str"
			node.Value = RedactedPlaceholder
			return 1
		}
		if strings.Contains(node.Value, "://") {
			if cleaned, did := RedactURL(node.Value); did {
				node.Tag = "!!str"
				node.Value = cleaned
				return 1
			}
		}
	}
	return count
}

// RestoreRedactedYAML merges incoming YAML with existing local YAML so any field equal to
// RedactedPlaceholder preserves the existing local non-empty credential when present.
func RestoreRedactedYAML(incoming, existing []byte) ([]byte, int, error) {
	if len(bytes.TrimSpace(incoming)) == 0 || len(bytes.TrimSpace(existing)) == 0 {
		return incoming, 0, nil
	}
	var inDoc, exDoc yaml.Node
	if err := yaml.Unmarshal(incoming, &inDoc); err != nil {
		return nil, 0, err
	}
	if err := yaml.Unmarshal(existing, &exDoc); err != nil {
		// Existing file is unreadable; fall back to incoming verbatim.
		return incoming, 0, nil
	}
	preserved := restoreYAMLNode(&inDoc, &exDoc)
	if preserved == 0 {
		return incoming, 0, nil
	}
	encoded, err := encodeYAMLNode(&inDoc)
	if err != nil {
		return nil, 0, err
	}
	return encoded, preserved, nil
}

func restoreYAMLNode(in, ex *yaml.Node) int {
	if in == nil || ex == nil {
		return 0
	}
	if in.Kind == yaml.DocumentNode && ex.Kind == yaml.DocumentNode && len(in.Content) > 0 && len(ex.Content) > 0 {
		return restoreYAMLNode(in.Content[0], ex.Content[0])
	}
	if in.Kind == yaml.ScalarNode && ex.Kind == yaml.ScalarNode {
		if ContainsRedacted(in.Value) && ex.Value != "" && !ContainsRedacted(ex.Value) {
			in.Value = ex.Value
			in.Tag = ex.Tag
			return 1
		}
		return 0
	}
	if in.Kind == yaml.MappingNode && ex.Kind == yaml.MappingNode {
		exMap := make(map[string]*yaml.Node, len(ex.Content)/2)
		for i := 0; i+1 < len(ex.Content); i += 2 {
			if ex.Content[i].Kind == yaml.ScalarNode {
				exMap[ex.Content[i].Value] = ex.Content[i+1]
			}
		}
		preserved := 0
		for i := 0; i+1 < len(in.Content); i += 2 {
			if in.Content[i].Kind != yaml.ScalarNode {
				continue
			}
			if match, ok := exMap[in.Content[i].Value]; ok {
				preserved += restoreYAMLNode(in.Content[i+1], match)
			}
		}
		return preserved
	}
	if in.Kind == yaml.SequenceNode && ex.Kind == yaml.SequenceNode {
		// Match named mapping entries (such as Mihomo proxies[]) by `name` first.
		exByName := make(map[string]*yaml.Node)
		for _, item := range ex.Content {
			if name := yamlMappingName(item); name != "" {
				exByName[name] = item
			}
		}
		preserved := 0
		for idx, item := range in.Content {
			if name := yamlMappingName(item); name != "" {
				if match, ok := exByName[name]; ok {
					preserved += restoreYAMLNode(item, match)
					continue
				}
			}
			if idx < len(ex.Content) {
				preserved += restoreYAMLNode(item, ex.Content[idx])
			}
		}
		return preserved
	}
	return 0
}

func yamlMappingName(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.MappingNode {
		return ""
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Kind == yaml.ScalarNode && n.Content[i].Value == "name" && n.Content[i+1].Kind == yaml.ScalarNode {
			return n.Content[i+1].Value
		}
	}
	return ""
}

func encodeYAMLNode(doc *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
