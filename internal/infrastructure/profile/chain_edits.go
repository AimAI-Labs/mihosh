package profile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	// ErrInvalidProxyChain indicates a structurally invalid proxy chain definition.
	ErrInvalidProxyChain = errors.New("invalid proxy chain configuration")
	// ErrChainProxyNotFound indicates a chain references a proxy missing from the configuration.
	ErrChainProxyNotFound = errors.New("chain proxy was not found in the effective configuration")
	// ErrChainCycle indicates enabled proxy chains form a circular dialer-proxy reference.
	ErrChainCycle = errors.New("proxy chain contains a circular dialer-proxy reference")
	// ErrChainConflict indicates two enabled chains assign different dialer-proxy targets to the same proxy.
	ErrChainConflict = errors.New("proxy chain conflicts with another enabled chain on the same proxy")
)

// ProxyChain defines an ordered entry -> ... -> exit proxy chain (Clash Verge Rev model).
// Mihomo realizes the chain by setting dialer-proxy on Nodes[i] to Nodes[i-1] for i >= 1,
// and selecting Nodes[len-1] (exit node) in Group.
type ProxyChain struct {
	Group   string   `yaml:"group,omitempty" json:"group,omitempty"`
	Nodes   []string `yaml:"nodes,omitempty" json:"nodes,omitempty"`
	Enabled bool     `yaml:"enabled" json:"enabled"`
}

// ChainEdits persists Mihosh-managed proxy chains independently of native merge.yaml.
type ChainEdits struct {
	Chains []ProxyChain `yaml:"chains,omitempty" json:"chains,omitempty"`
}

// ChainEditsPath returns the path to ~/.mihosh/chain-edits.yaml.
func ChainEditsPath() (string, error) {
	root, err := mihoshConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "chain-edits.yaml"), nil
}

// ReadChainEdits reads chain-edits.yaml; returns an empty ChainEdits if not present.
func ReadChainEdits() (ChainEdits, error) {
	path, err := ChainEditsPath()
	if err != nil {
		return ChainEdits{}, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ChainEdits{}, nil
	}
	if err != nil {
		return ChainEdits{}, err
	}
	return DecodeChainEdits(data)
}

// DecodeChainEdits parses chain-edits.yaml bytes.
func DecodeChainEdits(data []byte) (ChainEdits, error) {
	var edits ChainEdits
	if len(trimSpaceBytes(data)) == 0 {
		return edits, nil
	}
	if err := yaml.Unmarshal(data, &edits); err != nil {
		return edits, fmt.Errorf("read chain-edits.yaml: %w", err)
	}
	return edits, nil
}

// EncodeChainEdits serializes ChainEdits as 2-space indented YAML.
func EncodeChainEdits(edits ChainEdits) ([]byte, error) {
	return marshalYAML2Spaces(edits)
}

// WriteChainEdits atomically persists chain-edits.yaml with 0600 permissions.
func WriteChainEdits(edits ChainEdits) error {
	path, err := ChainEditsPath()
	if err != nil {
		return err
	}
	data, err := EncodeChainEdits(edits)
	if err != nil {
		return err
	}
	return atomicWrite(path, data, 0600)
}

// GroupChain returns the chain for groupName, if present.
func (e ChainEdits) GroupChain(groupName string) (ProxyChain, bool) {
	for _, c := range e.Chains {
		if c.Group == groupName {
			return c, true
		}
	}
	return ProxyChain{}, false
}

// SetGroupChain validates and upserts (or clears) a group's proxy chain.
func (e *ChainEdits) SetGroupChain(chain ProxyChain) error {
	chain.Group = strings.TrimSpace(chain.Group)
	cleaned := make([]string, 0, len(chain.Nodes))
	for _, n := range chain.Nodes {
		if t := strings.TrimSpace(n); t != "" {
			cleaned = append(cleaned, t)
		}
	}
	chain.Nodes = cleaned

	if err := ValidateProxyChain(chain); err != nil {
		return err
	}

	next := make([]ProxyChain, 0, len(e.Chains)+1)
	replaced := false
	for _, existing := range e.Chains {
		if existing.Group == chain.Group {
			if chain.Enabled && len(chain.Nodes) >= 2 {
				next = append(next, chain)
			}
			replaced = true
			continue
		}
		next = append(next, existing)
	}
	if !replaced && chain.Enabled && len(chain.Nodes) >= 2 {
		next = append(next, chain)
	}

	candidate := ChainEdits{Chains: next}
	if _, err := buildChainDialerMap(candidate); err != nil {
		return err
	}
	e.Chains = next
	return nil
}

// IsChainBuiltInProxy reports whether name is a built-in Mihomo special policy that cannot be chained.
func IsChainBuiltInProxy(name string) bool {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "DIRECT", "REJECT", "REJECT-DROP", "PASS", "COMPATIBLE":
		return true
	default:
		return false
	}
}

// ValidateProxyChain validates a single chain definition.
func ValidateProxyChain(chain ProxyChain) error {
	if !chain.Enabled && len(chain.Nodes) == 0 {
		return nil
	}
	if chain.Enabled && len(chain.Nodes) < 2 {
		return fmt.Errorf("%w: at least 2 nodes (entry and exit) are required", ErrInvalidProxyChain)
	}
	seen := make(map[string]struct{}, len(chain.Nodes))
	for _, node := range chain.Nodes {
		if node == "" {
			return fmt.Errorf("%w: empty node name", ErrInvalidProxyChain)
		}
		if IsChainBuiltInProxy(node) {
			return fmt.Errorf("%w: built-in policy %q cannot be used in a proxy chain", ErrInvalidProxyChain, node)
		}
		if _, dup := seen[node]; dup {
			return fmt.Errorf("%w: duplicate node %q in chain", ErrInvalidProxyChain, node)
		}
		seen[node] = struct{}{}
	}
	return nil
}

// buildChainDialerMap computes targetProxy -> dialerProxy from all enabled chains,
// rejecting conflicting targets and cross-chain cycles.
func buildChainDialerMap(edits ChainEdits) (map[string]string, error) {
	dialers := make(map[string]string)
	for _, chain := range edits.Chains {
		if !chain.Enabled || len(chain.Nodes) < 2 {
			continue
		}
		if err := ValidateProxyChain(chain); err != nil {
			return nil, err
		}
		for i := 1; i < len(chain.Nodes); i++ {
			target := chain.Nodes[i]
			dialer := chain.Nodes[i-1]
			if existing, exists := dialers[target]; exists && existing != dialer {
				return nil, fmt.Errorf("%w: %q already uses %q (cannot also use %q)", ErrChainConflict, target, existing, dialer)
			}
			dialers[target] = dialer
		}
	}
	// Detect cycles across all dialer edges.
	for start := range dialers {
		visited := make(map[string]bool)
		cur := start
		for {
			if visited[cur] {
				return nil, fmt.Errorf("%w: cycle involving %q", ErrChainCycle, cur)
			}
			visited[cur] = true
			next, ok := dialers[cur]
			if !ok {
				break
			}
			cur = next
		}
	}
	return dialers, nil
}

// ApplyChainEditsToYAML applies edits onto YAML bytes, optionally removing dialer-proxy
// assignments owned by previousEdits first (used when editing a standalone config without raw.yaml).
func ApplyChainEditsToYAML(data []byte, previousEdits, nextEdits ChainEdits, strict bool) ([]byte, error) {
	doc, err := parseOverrideDocument(data)
	if err != nil {
		return nil, err
	}
	root := topLevelMapping(doc)
	if err = revertChainDialers(root, previousEdits); err != nil {
		return nil, err
	}
	if err = applyChainEdits(root, nextEdits, strict); err != nil {
		return nil, err
	}
	return marshalYAML2Spaces(doc)
}

func revertChainDialers(root *yaml.Node, previous ChainEdits) error {
	if root == nil {
		return nil
	}
	prevMap, err := buildChainDialerMap(previous)
	if err != nil || len(prevMap) == 0 {
		return nil
	}
	proxies := resolveSequenceField(root, "proxies")
	if proxies == nil {
		return nil
	}
	for i, item := range proxies.Content {
		mapping := derefMapping(proxies, i, item)
		if mapping == nil {
			continue
		}
		name := mappingScalarValue(mapping, "name")
		prevDialer, managed := prevMap[name]
		if !managed {
			continue
		}
		if current := mappingScalarValue(mapping, "dialer-proxy"); current == prevDialer {
			deleteMappingField(mapping, "dialer-proxy")
		}
	}
	return nil
}

func applyChainEdits(root *yaml.Node, edits ChainEdits, strict bool) error {
	if root == nil {
		return nil
	}
	dialers, err := buildChainDialerMap(edits)
	if err != nil {
		return err
	}
	if len(dialers) == 0 {
		return nil
	}

	proxies := resolveSequenceField(root, "proxies")
	if proxies == nil {
		if strict {
			return fmt.Errorf("%w: no proxies section in configuration", ErrChainProxyNotFound)
		}
		return nil
	}

	// Collect available proxy and group names for strict validation.
	proxyIndex := make(map[string]int, len(proxies.Content))
	for i, item := range proxies.Content {
		m := item
		if m != nil && m.Kind == yaml.AliasNode {
			m = m.Alias
		}
		if m != nil && m.Kind == yaml.MappingNode {
			if name := mappingScalarValue(m, "name"); name != "" {
				proxyIndex[name] = i
			}
		}
	}

	if strict {
		groupNames := collectGroupNames(root)
		for _, chain := range edits.Chains {
			if !chain.Enabled || len(chain.Nodes) < 2 {
				continue
			}
			// Entry node may be a concrete proxy or a proxy-group; subsequent hops must be concrete proxies.
			entry := chain.Nodes[0]
			if _, ok := proxyIndex[entry]; !ok && !slices.Contains(groupNames, entry) {
				return fmt.Errorf("%w: entry node %q", ErrChainProxyNotFound, entry)
			}
			for _, hop := range chain.Nodes[1:] {
				if _, ok := proxyIndex[hop]; !ok {
					return fmt.Errorf("%w: %q is not a concrete proxy in proxies list", ErrChainProxyNotFound, hop)
				}
			}
		}
	}

	for target, dialer := range dialers {
		idx, ok := proxyIndex[target]
		if !ok {
			continue
		}
		mapping := derefMapping(proxies, idx, proxies.Content[idx])
		if mapping == nil {
			continue
		}
		setMappingField(mapping, "dialer-proxy", &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: dialer,
		})
	}
	return nil
}

func resolveSequenceField(root *yaml.Node, key string) *yaml.Node {
	list := findMappingValue(root, key)
	if list == nil || list.ShortTag() == "!!null" {
		return nil
	}
	if list.Kind == yaml.AliasNode {
		list = deepCopyNode(list.Alias)
		list.Anchor = ""
		setMappingField(root, key, list)
	}
	if list.Kind != yaml.SequenceNode {
		return nil
	}
	return list
}

func derefMapping(seq *yaml.Node, idx int, item *yaml.Node) *yaml.Node {
	if item == nil {
		return nil
	}
	if item.Kind == yaml.AliasNode {
		copied := deepCopyNode(item.Alias)
		copied.Anchor = ""
		seq.Content[idx] = copied
		item = copied
	}
	if item.Kind != yaml.MappingNode {
		return nil
	}
	return item
}

func mappingScalarValue(mapping *yaml.Node, key string) string {
	val := findMappingValue(mapping, key)
	if val == nil || val.Kind != yaml.ScalarNode {
		return ""
	}
	return val.Value
}

func deleteMappingField(mapping *yaml.Node, key string) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
			return
		}
	}
}

func collectGroupNames(root *yaml.Node) []string {
	groups := findMappingValue(root, "proxy-groups")
	if groups != nil && groups.Kind == yaml.AliasNode {
		groups = groups.Alias
	}
	if groups == nil || groups.Kind != yaml.SequenceNode {
		return nil
	}
	names := make([]string, 0, len(groups.Content))
	for _, item := range groups.Content {
		m := item
		if m != nil && m.Kind == yaml.AliasNode {
			m = m.Alias
		}
		if m != nil && m.Kind == yaml.MappingNode {
			if name := mappingScalarValue(m, "name"); name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}
