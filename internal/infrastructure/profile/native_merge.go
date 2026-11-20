package profile

import (
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Top-level configuration keys from Mihomo's config.RawConfig.
var nativeMergeFields = strings.Fields(`
	port socks-port redir-port tproxy-port mixed-port ss-config vmess-config
	inbound-tfo inbound-mptcp authentication skip-auth-prefixes lan-allowed-ips
	lan-disallowed-ips allow-lan bind-address mode unified-delay log-level ipv6
	external-controller external-controller-routing-mark external-controller-pipe
	external-controller-unix external-controller-tls external-controller-cors
	external-ui external-ui-url external-ui-name external-doh-server secret
	interface-name routing-mark tunnels geo-auto-update geo-update-interval
	geodata-mode geodata-loader geosite-matcher tcp-concurrent find-process-mode
	global-client-fingerprint global-ua etag-support keep-alive-idle keep-alive-interval
	disable-keep-alive proxy-providers rule-providers proxies proxy-groups rules
	sub-rules listeners hosts dns ntp tun tuic-server iptables experimental profile
	geox-url sniffer tls clash-for-android
`)

func legacyMergeTarget(key string) string {
	if strings.EqualFold(strings.TrimSpace(key), ruleEditsKey) {
		return "rules"
	}
	if kind, target := classifyMergeKey(key); kind != mergeKindReplace {
		return target
	}
	return ""
}

func validateMergeDocument(doc *yaml.Node, allowLegacy bool) error {
	root := topLevelMapping(doc)
	if root == nil {
		return fmt.Errorf("merge.yaml must contain a YAML mapping")
	}
	seen := make(map[string]bool, len(root.Content)/2)
	for i := 0; i < len(root.Content); i += 2 {
		key := root.Content[i]
		if key.Kind != yaml.ScalarNode || key.ShortTag() != "!!str" {
			return fmt.Errorf("merge.yaml field at line %d must be a string", key.Line)
		}
		if seen[key.Value] {
			return fmt.Errorf("duplicate merge.yaml field %q", key.Value)
		}
		seen[key.Value] = true
		if allowLegacy && legacyMergeTarget(key.Value) != "" {
			continue
		}
		switch key.Value {
		case "rules":
			var rules []string
			if err := root.Content[i+1].Decode(&rules); err != nil {
				return fmt.Errorf("merge.yaml rules must be a list of strings: %w", err)
			}
		case "proxies", "proxy-groups", "listeners":
			var entries []map[string]any
			if err := root.Content[i+1].Decode(&entries); err != nil {
				return fmt.Errorf("merge.yaml %s must be a list of mappings: %w", key.Value, err)
			}
		}
		if !slices.Contains(nativeMergeFields, key.Value) {
			return fmt.Errorf("merge.yaml field %q is not a native Mihomo field", key.Value)
		}
	}
	return nil
}

// NormalizeLegacyMerge converts legacy list operations using the active source.
// Without a subscription, path is the current local Mihomo configuration.
func NormalizeLegacyMerge(data []byte, uid string, localPath func() (string, error)) ([]byte, error) {
	doc, err := parseOverrideDocument(data)
	if err != nil {
		return nil, err
	}
	root := topLevelMapping(doc)
	affected := make(map[string]bool, 3)
	for i := 0; i < len(root.Content); i += 2 {
		if target := legacyMergeTarget(root.Content[i].Value); target != "" && target != "rules" {
			affected[target] = true
		}
	}
	hasLegacyRules, _ := legacyRuleOverrides(root)
	if len(affected) == 0 {
		if !hasLegacyRules {
			return data, nil
		}
		removeLegacyFields(root)
		return marshalYAML2Spaces(doc)
	}

	// The rule directives move to rule-edits.yaml, independent of this source.
	for i := len(root.Content) - 2; i >= 0; i -= 2 {
		key := root.Content[i].Value
		if legacyMergeTarget(key) == "rules" {
			removeOverride(root, key)
		}
	}
	var raw *yaml.Node
	if uid != "" {
		raw, err = ReadRaw(uid)
	} else {
		var path string
		path, err = localPath()
		if err == nil {
			var runtime []byte
			runtime, err = os.ReadFile(path)
			if err == nil {
				raw, err = parseOverrideDocument(runtime)
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("converting legacy merge.yaml requires the active subscription or a local Mihomo config: %w", err)
	}
	if topLevelMapping(raw) == nil {
		return nil, fmt.Errorf("converting legacy merge.yaml requires a Mihomo configuration mapping")
	}
	overlay := root
	if uid == "" {
		// The local config may already contain the previous list operations.
		overlay = &yaml.Node{Kind: yaml.MappingNode}
		for i := 0; i < len(root.Content); i += 2 {
			key, value := root.Content[i], root.Content[i+1]
			kind, target := classifyMergeKey(key.Value)
			if kind != mergeKindReplace && findMappingValue(root, target) == nil &&
				listAlreadyMerged(findMappingValue(topLevelMapping(raw), target), value, kind) {
				continue
			}
			overlay.Content = append(overlay.Content, key, value)
		}
	}
	effective, err := applyMerge(raw, overlay, uid != "")
	if err != nil {
		return nil, err
	}
	removeLegacyFields(root)
	for _, key := range []string{"proxies", "proxy-groups"} {
		if affected[key] {
			if err := replaceMergeList(root, key, findMappingValue(effective, key)); err != nil {
				return nil, err
			}
		}
	}
	return marshalYAML2Spaces(doc)
}

func listAlreadyMerged(current, additional *yaml.Node, kind mergeKind) bool {
	if current == nil || current.Kind != yaml.SequenceNode || additional.Kind != yaml.SequenceNode ||
		len(additional.Content) > len(current.Content) {
		return false
	}
	start := 0
	if kind == mergeKindAppend {
		start = len(current.Content) - len(additional.Content)
	}
	for i, entry := range additional.Content {
		var a, b any
		if entry.Decode(&a) != nil || current.Content[start+i].Decode(&b) != nil || !reflect.DeepEqual(a, b) {
			return false
		}
	}
	return true
}

func removeLegacyFields(root *yaml.Node) {
	for i := len(root.Content) - 2; i >= 0; i -= 2 {
		key := root.Content[i].Value
		if legacyMergeTarget(key) != "" {
			removeOverride(root, key)
		}
	}
}

func replaceMergeList(root *yaml.Node, key string, source *yaml.Node) error {
	list := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	if source != nil {
		// Expand aliases since the referenced anchors can belong to the subscription.
		var entries []any
		if err := source.Decode(&entries); err != nil {
			return fmt.Errorf("merge.yaml %s: %w", key, err)
		}
		if err := list.Encode(entries); err != nil {
			return err
		}
	}
	if old := findMappingValue(root, key); old != nil {
		list.Anchor, list.HeadComment, list.LineComment, list.FootComment = old.Anchor, old.HeadComment, old.LineComment, old.FootComment
		*old = *list
	} else {
		setMappingField(root, key, list)
	}
	return nil
}
