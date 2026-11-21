package sync

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"path/filepath"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/stretchr/testify/require"
)

func TestRedactURLAndDSN(t *testing.T) {
	// URL with userinfo + sensitive query param + UUID path segment + benign param
	raw := "https://alice:topsecret@sub.example.com/link/550e8400-e29b-41d4-a716-446655440000/clash?token=abc123xyz&flag=clash-meta"
	got, did := RedactURL(raw)
	require.True(t, did)
	require.NotContains(t, got, "topsecret")
	require.NotContains(t, got, "abc123xyz")
	require.NotContains(t, got, "550e8400-e29b-41d4-a716-446655440000")
	require.Contains(t, got, "flag=clash-meta")
	require.Contains(t, got, RedactedPlaceholder)

	// URL with long opaque hex token in path
	raw2 := "https://provider.example.org/api/v1/client/subscribe/0123456789abcdef0123456789abcdef"
	got2, did2 := RedactURL(raw2)
	require.True(t, did2)
	require.NotContains(t, got2, "0123456789abcdef")
	require.Contains(t, got2, "/subscribe/[REDACTED]")

	// Benign URL stays untouched
	benign := "https://example.com/public/config.yaml?flag=meta"
	got3, did3 := RedactURL(benign)
	require.False(t, did3)
	require.Equal(t, benign, got3)

	// PostgreSQL URI and KV DSN redaction
	require.Equal(t,
		"postgres://mihosh:[REDACTED]@db.example.com:5432/mihosh?sslmode=require",
		RedactDSN("postgres://mihosh:s3cr3t!@db.example.com:5432/mihosh?sslmode=require"),
	)
	require.Equal(t,
		"host=db.example.com user=mihosh password=[REDACTED] dbname=mihosh",
		RedactDSN("host=db.example.com user=mihosh password=supersecret dbname=mihosh"),
	)
}

func TestRedactYAMLAndRestorePreservesLocalSecrets(t *testing.T) {
	mergeYAML := []byte(`mixed-port: 7890
allow-lan: false
secret: controller-secret-123
authentication:
  - admin:pass123
proxies:
  - name: NodeA
    type: vmess
    server: 1.2.3.4
    port: 443
    uuid: 550e8400-e29b-41d4-a716-446655440000
    password: proxy-password-xyz
`)
	redacted, count, err := RedactYAML(mergeYAML)
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, 4)
	require.NotContains(t, string(redacted), "controller-secret-123")
	require.NotContains(t, string(redacted), "admin:pass123")
	require.NotContains(t, string(redacted), "550e8400-e29b-41d4-a716-446655440000")
	require.NotContains(t, string(redacted), "proxy-password-xyz")
	require.Contains(t, string(redacted), "mixed-port: 7890")
	require.Contains(t, string(redacted), RedactedPlaceholder)

	// Incoming redacted YAML updates mixed-port to 7891 while preserving local secrets on restore.
	incoming := bytes.ReplaceAll(redacted, []byte("7890"), []byte("7891"))
	restored, preserved, err := RestoreRedactedYAML(incoming, mergeYAML)
	require.NoError(t, err)
	require.GreaterOrEqual(t, preserved, 4)
	require.Contains(t, string(restored), "mixed-port: 7891")
	require.Contains(t, string(restored), "controller-secret-123")
	require.Contains(t, string(restored), "proxy-password-xyz")
}

func TestBundleTarGzAndZipRoundTripAndSecurity(t *testing.T) {
	cfg := &config.Config{
		TestURL:             "https://cp.cloudflare.com/generate_204",
		Timeout:             3000,
		Language:            "zh-CN",
		AutoRefreshInterval: 10,
		Theme:               "catppuccin",
		BorderStyle:         "square",
		ActiveSub:           "sub-1",
		Core: config.CoreConfig{
			ExternalSecret: "must-never-leak",
		},
		Sync: config.SyncConfig{
			DSN: "postgres://u:pgpass@host/db",
		},
		Subs: []profile.Profile{
			{
				UID:  "sub-1",
				Name: "MainSub",
				Source: profile.SubSource{
					Kind:               profile.SourceRemote,
					URL:                "https://sub.example.com/sub?token=secret999&flag=meta",
					UseProxy:           true,
					UserAgent:          "mihosh/2.0",
					InsecureSkipVerify: false,
					UpdateInterval:     720,
				},
			},
		},
	}

	bundle, err := BuildBundle(
		cfg,
		[]byte("secret: ctrl-key\nlog-level: info\n"),
		[]byte("edits:\n  - rule: DOMAIN,example.com,DIRECT\n"),
		[]byte("chains:\n  - group: PROXY\n    nodes: [HK, US]\n    enabled: true\n"),
		true,
	)
	require.NoError(t, err)
	require.True(t, bundle.Manifest.Redacted)
	require.GreaterOrEqual(t, bundle.Manifest.RedactedCount, 2)
	require.NotContains(t, bundle.Config.Subs[0].Source.URL, "secret999")
	require.Contains(t, bundle.Config.Subs[0].Source.URL, RedactedPlaceholder)
	require.NotContains(t, string(bundle.MergeYAML), "ctrl-key")

	// Round-trip .tar.gz file
	tmpDir := t.TempDir()
	tgzPath := filepath.Join(tmpDir, "backup.tar.gz")
	n, err := WriteArchiveFile(tgzPath, bundle)
	require.NoError(t, err)
	require.Greater(t, n, 0)

	decodedTgz, err := ReadArchiveFile(tgzPath)
	require.NoError(t, err)
	require.Equal(t, "catppuccin", decodedTgz.Config.Preferences.Theme)
	require.Equal(t, "square", decodedTgz.Config.Preferences.BorderStyle)
	require.Len(t, decodedTgz.Config.Subs, 1)
	require.Equal(t, 720, decodedTgz.Config.Subs[0].Source.UpdateInterval)
	require.Contains(t, string(decodedTgz.RuleEdits), "DOMAIN,example.com,DIRECT")
	require.Contains(t, string(decodedTgz.ChainEdits), "group: PROXY")

	// Round-trip .zip file
	zipPath := filepath.Join(tmpDir, "backup.zip")
	_, err = WriteArchiveFile(zipPath, bundle)
	require.NoError(t, err)
	decodedZip, err := ReadArchiveFile(zipPath)
	require.NoError(t, err)
	h1, err := ContentSHA256(decodedTgz)
	require.NoError(t, err)
	h2, err := ContentSHA256(decodedZip)
	require.NoError(t, err)
	require.Equal(t, h1, h2)

	// Reject tar-slip path traversal
	var maliciousTar bytes.Buffer
	gz := gzip.NewWriter(&maliciousTar)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: "../evil.yaml", Mode: 0600, Size: 3})
	_, _ = tw.Write([]byte("bad"))
	_ = tw.Close()
	_ = gz.Close()
	_, err = DecodeTarGz(maliciousTar.Bytes())
	require.ErrorIs(t, err, ErrInvalidArchive)

	// Reject zip-slip path traversal
	var maliciousZip bytes.Buffer
	zw := zip.NewWriter(&maliciousZip)
	w, _ := zw.Create("../../evil.yaml")
	_, _ = w.Write([]byte("bad"))
	_ = zw.Close()
	_, err = DecodeZip(maliciousZip.Bytes())
	require.ErrorIs(t, err, ErrInvalidArchive)
}
