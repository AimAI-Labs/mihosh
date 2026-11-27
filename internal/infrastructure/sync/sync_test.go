package sync

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
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

func TestRestoreRedactedYAML_NamedSequenceDoesNotLeakAcrossProxies(t *testing.T) {
	existing := []byte(`proxies:
  - name: Private-JP
    server: jp.internal
    password: secret-jp-password
`)
	incoming := []byte(`proxies:
  - name: New-US
    server: us.thirdparty
    password: "[REDACTED]"
  - name: Private-JP
    server: jp.internal
    password: "[REDACTED]"
`)
	restored, count, err := RestoreRedactedYAML(incoming, existing)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	out := string(restored)
	require.Contains(t, out, "name: Private-JP")
	require.Contains(t, out, "secret-jp-password")
	// New-US must NOT inherit Private-JP's password!
	require.Equal(t, 1, bytes.Count(restored, []byte("secret-jp-password")))
	require.Contains(t, out, `password: "[REDACTED]"`)
}

func TestRedactDSN_FallbackAndQueryParams(t *testing.T) {
	// A malformed URI must never be returned as display text.
	badURI := "postgres://admin:p%ss@db.example:5432/app"
	redactedBad := RedactDSN(badURI)
	require.NotContains(t, redactedBad, "p%ss")
	require.Contains(t, redactedBad, RedactedPlaceholder)

	// Query parameters carrying password or sslpassword must be redacted too.
	queryURI := "postgres://admin@db.example:5432/app?sslmode=require&password=topsecret&sslpassword=keypass"
	redactedQuery := RedactDSN(queryURI)
	require.NotContains(t, redactedQuery, "topsecret")
	require.NotContains(t, redactedQuery, "keypass")
	require.Contains(t, redactedQuery, "sslmode=require")
}

func TestDecodeArchive_RejectsDuplicateEntries(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, content := range []string{"version: 1\n", "version: 1\n"} {
		_ = tw.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0600, Size: int64(len(content))})
		_, _ = tw.Write([]byte(content))
	}
	_ = tw.Close()
	_ = gz.Close()
	_, err := DecodeTarGz(buf.Bytes())
	require.ErrorIs(t, err, ErrInvalidArchive)
}

func TestOpenPostgresDoesNotExposeMalformedURIPassword(t *testing.T) {
	for _, password := range []string{"p%ss", "p%ss/part@tail"} {
		dsn := "postgres://admin:" + password + "@127.0.0.1:1/app"
		store, err := OpenPostgres(context.Background(), dsn)
		require.Nil(t, store)
		require.Error(t, err)
		require.NotContains(t, err.Error(), password)
		require.NotContains(t, err.Error(), dsn)
		require.Contains(t, err.Error(), "postgres")
	}
}

func TestRedactDSNDoesNotExposeMalformedURIPassword(t *testing.T) {
	for _, dsn := range []string{
		"postgres://admin:p%ss/part@tail@db.example:5432/app",
		"postgresql://admin:p%ss@second@db.example:5432/app",
		"postgres://admin:123/part@db.example:5432/app",
		"postgres://db.example:5432/app?password=p%ss",
	} {
		require.Equal(t, RedactedPlaceholder, RedactDSN(dsn))
	}
	require.Equal(t, "host=db.example password=[REDACTED]", RedactDSN("host=db.example password='part://other'"))
}

func TestRedactURLIdentifiesTokensInConfigFilenames(t *testing.T) {
	const prefix = "https://sub.example.com/clash/"
	for _, name := range []string{
		"550e8400-e29b-41d4-a716-446655440000.yaml",
		"0123456789abcdef0123456789abcdef.yml",
		"ExampleOpaqueKeyNumber12345.JSON",
	} {
		got, changed := RedactURL(prefix + name)
		require.True(t, changed, name)
		require.Equal(t, prefix+RedactedPlaceholder, got)
	}
	for _, name := range []string{"subscribe.yaml", "clash-config.yml"} {
		got, changed := RedactURL(prefix + name)
		require.False(t, changed, name)
		require.Equal(t, prefix+name, got)
	}
}

func TestRedactYAMLKeepsSkipAuthPrefixesOnRestore(t *testing.T) {
	original := []byte("skip-auth-prefixes: [127.0.0.1/8, '::1/128']\nauthentication: ['user:password']\nsecret: private\n")
	redacted, count, err := RedactYAML(original)
	require.NoError(t, err)
	require.Equal(t, 2, count)
	require.Contains(t, string(redacted), "127.0.0.1/8")
	require.Contains(t, string(redacted), "::1/128")
	require.NotContains(t, string(redacted), "user:password")
	require.NotContains(t, string(redacted), "private")

	restored, count, err := RestoreRedactedYAML(redacted, original)
	require.NoError(t, err)
	require.Equal(t, 2, count)
	require.Contains(t, string(restored), "127.0.0.1/8")
	require.Contains(t, string(restored), "::1/128")
	require.Contains(t, string(restored), "user:password")
	require.Contains(t, string(restored), "private")
}
