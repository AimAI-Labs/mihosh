package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestReadMihomoOverridesRoundTripAndEdgeCases(t *testing.T) {
	mockMihoshConfigDir(t)

	// Missing merge.yaml -> empty map, no error.
	got, err := ReadMihomoOverrides()
	require.NoError(t, err)
	assert.Empty(t, got)

	// Null-only and comment-only merge.yaml -> empty map.
	for _, raw := range []string{"# only comment\n", "---\n", "null\n"} {
		values, err := ReadMihomoOverridesFrom([]byte(raw))
		require.NoError(t, err, "input=%q", raw)
		assert.Empty(t, values)
	}

	// Reads all five fields including explicit allow-lan: false and skips null keys.
	require.NoError(t, WriteMerge([]byte(`# top comment
external-controller: 127.0.0.1:9091
secret: s3cr3t
mixed-port: 7891
allow-lan: false
log-level: debug
prepend-rules:
  - DOMAIN,example.com,DIRECT
`)))
	got, err = ReadMihomoOverrides()
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"external-controller": "127.0.0.1:9091",
		"secret":              "s3cr3t",
		"mixed-port":          "7891",
		"allow-lan":           "false",
		"log-level":           "debug",
	}, got)

	// Null value is treated as unset.
	got, err = ReadMihomoOverridesFrom([]byte("allow-lan: null\nlog-level: ~\n"))
	require.NoError(t, err)
	assert.Empty(t, got)

	// Invalid YAML or non-mapping document errors.
	_, err = ReadMihomoOverridesFrom([]byte("rules: [\n"))
	require.Error(t, err)
	_, err = ReadMihomoOverridesFrom([]byte("- not-a-map\n"))
	require.Error(t, err)
	_, err = ReadMihomoOverridesFrom([]byte("secret: [bad]\n"))
	require.Error(t, err)
}

func TestEncodeMihomoOverridesPreservesOtherFieldsExplicitFalseAndRemovesClearedKeys(t *testing.T) {
	initial := []byte(`# keep header
mode: rule
dns:
  enable: true
  nameserver:
    - 1.1.1.1
# lan comment
allow-lan: true # inline
log-level: warning
prepend-rules:
  - DOMAIN,keep.example,DIRECT
`)

	// Set explicit allow-lan: false, remove log-level, add mixed-port & controller & secret.
	encoded, err := EncodeMihomoOverrides(initial, map[string]string{
		"external-controller": "127.0.0.1:9092",
		"secret":              "my-secret",
		"mixed-port":          "7895",
		"allow-lan":           "false",
		"log-level":           "",
	})
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal(encoded, &parsed))
	assert.Equal(t, "rule", parsed["mode"])
	assert.Contains(t, parsed, "dns")
	assert.Contains(t, parsed, "prepend-rules")
	assert.Equal(t, "127.0.0.1:9092", parsed["external-controller"])
	assert.Equal(t, "my-secret", parsed["secret"])
	assert.Equal(t, 7895, parsed["mixed-port"])
	assert.Equal(t, false, parsed["allow-lan"])
	assert.NotContains(t, parsed, "log-level", "empty log-level must remove the key")
	assert.Contains(t, string(encoded), "# keep header")
	assert.Contains(t, string(encoded), "# inline")

	// Reading back yields explicit "false" for allow-lan and absent log-level.
	readBack, err := ReadMihomoOverridesFrom(encoded)
	require.NoError(t, err)
	assert.Equal(t, "false", readBack["allow-lan"])
	_, hasLogLevel := readBack["log-level"]
	assert.False(t, hasLogLevel)

	// Clearing allow-lan removes allow-lan while keeping other YAML entries.
	cleared, err := EncodeMihomoOverrides(encoded, map[string]string{
		"external-controller": "",
		"secret":              "",
		"mixed-port":          "",
		"allow-lan":           "",
		"log-level":           "",
	})
	require.NoError(t, err)
	var afterClear map[string]any
	require.NoError(t, yaml.Unmarshal(cleared, &afterClear))
	for _, k := range managedFields {
		assert.NotContains(t, afterClear, k)
	}
	assert.Equal(t, "rule", afterClear["mode"])
	assert.Contains(t, afterClear, "dns")
	assert.Contains(t, afterClear, "prepend-rules")

	// Clearing all keys from an overrides-only document without comments returns empty bytes.
	emptyOut, err := EncodeMihomoOverrides([]byte("allow-lan: false\nlog-level: info\n"), map[string]string{})
	require.NoError(t, err)
	assert.Empty(t, emptyOut)
}

func TestEncodeMihomoOverridesValidation(t *testing.T) {
	cases := []struct {
		name      string
		overrides map[string]string
		wantErr   bool
	}{
		{"valid IPv4 controller", map[string]string{"external-controller": "127.0.0.1:9090"}, false},
		{"valid IPv6 controller", map[string]string{"external-controller": "[::1]:9090"}, false},
		{"valid hostname controller", map[string]string{"external-controller": "localhost:9090"}, false},
		{"controller missing port", map[string]string{"external-controller": "127.0.0.1"}, true},
		{"controller with scheme", map[string]string{"external-controller": "http://127.0.0.1:9090"}, true},
		{"controller port zero", map[string]string{"external-controller": "127.0.0.1:0"}, true},
		{"controller port overflow", map[string]string{"external-controller": "127.0.0.1:70000"}, true},
		{"controller empty host", map[string]string{"external-controller": ":9090"}, true},
		{"mixed-port valid", map[string]string{"mixed-port": "7890"}, false},
		{"mixed-port zero", map[string]string{"mixed-port": "0"}, true},
		{"mixed-port invalid", map[string]string{"mixed-port": "abc"}, true},
		{"allow-lan invalid", map[string]string{"allow-lan": "yes"}, true},
		{"log-level invalid", map[string]string{"log-level": "trace"}, true},
		{"log-level silent", map[string]string{"log-level": "silent"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := EncodeMihomoOverrides(nil, tc.overrides)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGenerateOverrideConfigManagedAndExternalPrecedence(t *testing.T) {
	mockMihoshConfigDir(t)

	uid := "sub1"
	rawSource := []byte(`external-controller: 127.0.0.1:19090
secret: sub-secret
mixed-port: 17890
allow-lan: true
log-level: warning
mode: rule
rules:
  - MATCH,DIRECT
`)
	require.NoError(t, WriteRaw(uid, rawSource))

	currentPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(currentPath, []byte(`external-controller: 127.0.0.1:9090
secret: local-secret
mixed-port: 7890
allow-lan: true
log-level: debug
mode: rule
`), 0600))

	// 1. Managed + active sub + empty merge: keeps controller/secret/mixed-port from current file,
	// enforces safe allow-lan: false (ignoring untrusted sub's allow-lan: true), and lets log-level fall through to raw.
	out, err := GenerateOverrideConfigWithMerge(uid, currentPath, nil, []string{"allow-lan", "log-level"}, true)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, yaml.Unmarshal(out, &m))
	assert.Equal(t, "127.0.0.1:9090", m["external-controller"])
	assert.Equal(t, "local-secret", m["secret"])
	assert.Equal(t, 7890, m["mixed-port"])
	assert.Equal(t, false, m["allow-lan"])
	assert.Equal(t, "warning", m["log-level"])

	// 2. Managed + active sub + explicit allow-lan: true and new controller/secret in merge: explicit overrides win.
	mergeBytes := []byte("external-controller: 127.0.0.1:9099\nsecret: override-secret\nallow-lan: true\nlog-level: error\n")
	out, err = GenerateOverrideConfigWithMerge(uid, currentPath, mergeBytes, nil, true)
	require.NoError(t, err)
	m = nil
	require.NoError(t, yaml.Unmarshal(out, &m))
	assert.Equal(t, "127.0.0.1:9099", m["external-controller"])
	assert.Equal(t, "override-secret", m["secret"])
	assert.Equal(t, 7890, m["mixed-port"])
	assert.Equal(t, true, m["allow-lan"])
	assert.Equal(t, "error", m["log-level"])

	// 3. External + active sub + empty allow-lan/log-level: falls through to raw.yaml (allow-lan: true, log-level: warning).
	out, err = GenerateOverrideConfigWithMerge(uid, currentPath, nil, []string{"allow-lan", "log-level"}, false)
	require.NoError(t, err)
	m = nil
	require.NoError(t, yaml.Unmarshal(out, &m))
	assert.Equal(t, true, m["allow-lan"])
	assert.Equal(t, "warning", m["log-level"])
	assert.Equal(t, "127.0.0.1:9090", m["external-controller"])

	// 4. External + active sub + explicit allow-lan: false overrides raw.yaml's allow-lan: true.
	out, err = GenerateOverrideConfigWithMerge(uid, currentPath, []byte("allow-lan: false\n"), nil, false)
	require.NoError(t, err)
	m = nil
	require.NoError(t, yaml.Unmarshal(out, &m))
	assert.Equal(t, false, m["allow-lan"])

	// 5. No active sub (uid == ""): cleared log-level/allow-lan are removed from current config file.
	out, err = GenerateOverrideConfigWithMerge("", currentPath, []byte("mixed-port: 7899\n"), []string{"log-level", "allow-lan"}, false)
	require.NoError(t, err)
	m = nil
	require.NoError(t, yaml.Unmarshal(out, &m))
	assert.Equal(t, 7899, m["mixed-port"])
	assert.Equal(t, "127.0.0.1:9090", m["external-controller"])
	assert.NotContains(t, m, "log-level")
	assert.NotContains(t, m, "allow-lan")

	// 6. Remote-only (path == "") with active sub uses raw.yaml + merge.
	out, err = GenerateOverrideConfigWithMerge(uid, "", []byte("allow-lan: false\nlog-level: silent\n"), nil, false)
	require.NoError(t, err)
	m = nil
	require.NoError(t, yaml.Unmarshal(out, &m))
	assert.Equal(t, false, m["allow-lan"])
	assert.Equal(t, "silent", m["log-level"])
	assert.Equal(t, 17890, m["mixed-port"])

	// Subscription raw snapshot remains strictly read-only.
	rawAfter, err := ReadRawFile(uid)
	require.NoError(t, err)
	assert.Equal(t, rawSource, rawAfter)
}
