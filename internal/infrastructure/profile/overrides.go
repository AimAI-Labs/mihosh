package profile

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/netip"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ReadMihomoOverrides reads the five editable fields from merge.yaml.
func ReadMihomoOverrides() (map[string]string, error) {
	current, err := ReadMerge()
	if err != nil {
		return nil, err
	}
	return ReadMihomoOverridesFrom(current)
}

// ReadMihomoOverridesFrom reads the five editable fields from raw merge.yaml bytes.
func ReadMihomoOverridesFrom(current []byte) (map[string]string, error) {
	doc, err := parseOverrideDocument(current)
	if err != nil {
		return nil, err
	}
	mapping := topLevelMapping(doc)
	values := make(map[string]string, len(managedFields))
	for _, key := range managedFields {
		node := findMappingValue(mapping, key)
		if node == nil || node.ShortTag() == "!!null" {
			continue
		}
		var value string
		if err := node.Decode(&value); err != nil {
			return nil, fmt.Errorf("invalid %s override", key)
		}
		switch {
		case key == "allow-lan" && node.ShortTag() == "!!bool":
			var enabled bool
			if node.Decode(&enabled) == nil {
				value = strconv.FormatBool(enabled)
			}
		case key == "mixed-port" && node.ShortTag() == "!!int":
			var port int
			if node.Decode(&port) == nil {
				value = strconv.Itoa(port)
			}
		}
		if key != "secret" {
			value = strings.TrimSpace(value)
		}
		if value != "" {
			values[key] = value
		}
	}
	return values, nil
}

// ValidateMihomoOverride validates a single override value; empty values mean unset.
func ValidateMihomoOverride(key, value string) error {
	if key != "secret" {
		value = strings.TrimSpace(value)
	}
	if value == "" {
		return nil
	}
	_, err := encodeOverride(key, value)
	return err
}

// ClearedOverrideKeys returns managed override keys that were present in before and empty in after.
func ClearedOverrideKeys(before, after map[string]string) []string {
	var cleared []string
	for _, key := range managedFields {
		prev := before[key]
		next := after[key]
		if key != "secret" {
			prev = strings.TrimSpace(prev)
			next = strings.TrimSpace(next)
		}
		if prev != "" && next == "" {
			cleared = append(cleared, key)
		}
	}
	return cleared
}

// EncodeMihomoOverrides replaces the five editable overrides while retaining other YAML entries.
func EncodeMihomoOverrides(current []byte, overrides map[string]string) ([]byte, error) {
	doc, err := parseOverrideDocument(current)
	if err != nil {
		return nil, err
	}
	mapping := topLevelMapping(doc)
	for _, key := range managedFields {
		value := overrides[key]
		if key != "secret" {
			value = strings.TrimSpace(value)
		}
		if value == "" {
			removeOverride(mapping, key)
			continue
		}
		node, err := encodeOverride(key, value)
		if err != nil {
			return nil, err
		}
		if previous := findMappingValue(mapping, key); previous != nil {
			node.Anchor = previous.Anchor
			node.HeadComment = previous.HeadComment
			node.LineComment = previous.LineComment
			node.FootComment = previous.FootComment
			*previous = node
		} else {
			setMappingField(mapping, key, &node)
		}
	}
	if len(mapping.Content) == 0 &&
		doc.HeadComment == "" && doc.FootComment == "" &&
		mapping.HeadComment == "" && mapping.LineComment == "" && mapping.FootComment == "" {
		return nil, nil
	}
	out, err := marshalYAML2Spaces(doc)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(out, &yaml.Node{}); err != nil {
		return nil, fmt.Errorf("invalid resulting override YAML: %w", err)
	}
	return out, nil
}

func parseOverrideDocument(current []byte) (*yaml.Node, error) {
	var doc yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(current))
	if err := decoder.Decode(&doc); err == io.EOF {
		mapping := &yaml.Node{Kind: yaml.MappingNode, HeadComment: strings.TrimSpace(string(current))}
		return &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{mapping}}, nil
	} else if err != nil {
		return nil, fmt.Errorf("invalid override YAML: %w", err)
	}
	if err := decoder.Decode(&yaml.Node{}); err != io.EOF {
		return nil, fmt.Errorf("merge.yaml must contain one YAML mapping")
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) == 1 && doc.Content[0].Kind == yaml.ScalarNode && doc.Content[0].ShortTag() == "!!null" {
		n := doc.Content[0]
		doc.Content[0] = &yaml.Node{
			Kind:        yaml.MappingNode,
			HeadComment: n.HeadComment,
			LineComment: n.LineComment,
			FootComment: n.FootComment,
		}
	}
	if topLevelMapping(&doc) == nil {
		return nil, fmt.Errorf("merge.yaml must contain a YAML mapping")
	}
	return &doc, nil
}

func removeOverride(mapping *yaml.Node, key string) {
	for i := 0; i < len(mapping.Content); {
		entry := mapping.Content[i]
		if entry.Kind != yaml.ScalarNode || entry.Value != key {
			i += 2
			continue
		}
		if entry.HeadComment != "" {
			if i+2 < len(mapping.Content) {
				next := mapping.Content[i+2]
				next.HeadComment = strings.TrimSpace(entry.HeadComment + "\n" + next.HeadComment)
			} else {
				mapping.FootComment = strings.TrimSpace(entry.HeadComment + "\n" + mapping.FootComment)
			}
		}
		mapping.Content = slices.Delete(mapping.Content, i, i+2)
	}
}

func encodeOverride(key, value string) (yaml.Node, error) {
	var typed any = value
	switch key {
	case "external-controller":
		host, port, err := net.SplitHostPort(value)
		number, portErr := strconv.ParseUint(port, 10, 16)
		if err != nil || portErr != nil || number == 0 || !validControllerHost(host) {
			return yaml.Node{}, fmt.Errorf("external-controller must be a host and port (1–65535)")
		}
	case "mixed-port":
		port, err := strconv.ParseUint(value, 10, 16)
		if err != nil || port == 0 {
			return yaml.Node{}, fmt.Errorf("mixed-port must be between 1 and 65535")
		}
		typed = int(port)
	case "allow-lan":
		if value != "true" && value != "false" {
			return yaml.Node{}, fmt.Errorf("allow-lan must be true or false")
		}
		typed = value == "true"
	case "log-level":
		switch value {
		case "info", "warning", "error", "debug", "silent":
		default:
			return yaml.Node{}, fmt.Errorf("invalid log-level override")
		}
	}
	var node yaml.Node
	err := node.Encode(typed)
	return node, err
}

var controllerDNSLabel = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

func validControllerHost(host string) bool {
	if _, err := netip.ParseAddr(host); err == nil {
		return true
	}
	host = strings.TrimSuffix(host, ".")
	if len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if !controllerDNSLabel.MatchString(label) {
			return false
		}
	}
	return true
}
