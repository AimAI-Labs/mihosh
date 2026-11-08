package core

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestReleaseLocksTagAndRejectsGatewayDigest(t *testing.T) {
	digest := strings.Repeat("a", 64)
	json := fmt.Sprintf(`{"tag_name":"v1.2.3","assets":[{"name":"mihomo-linux-amd64-compatible-v1.2.3.gz","browser_download_url":"https://github.com/MetaCubeX/mihomo/releases/download/v1.2.3/mihomo-linux-amd64-compatible-v1.2.3.gz","digest":"sha256:%s","size":123}]}`, digest)
	f := testFetcher(t, func(route Route, _ *http.Request) (int, string, error) {
		if route.Name == "direct" {
			return 503, "", nil
		}
		return 200, json, nil
	})
	release, err := f.Release(context.Background(), "v1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	asset, err := release.SelectAsset("linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = release.TrustedDigest(asset, nil); err == nil {
		t.Fatal("gateway-supplied digest was trusted")
	}
	pins := map[string]string{"v1.2.3/" + asset.Name: digest}
	if got, err := release.TrustedDigest(asset, pins); err != nil || got != digest {
		t.Fatalf("%s %v", got, err)
	}
	release.Trusted = true
	if _, err = release.TrustedDigest(asset, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = release.SelectAsset("linux", "386"); err == nil {
		t.Fatal("unsupported CPU accepted")
	}
}
func TestReleaseRejectsWrongTagAndAssetOrigin(t *testing.T) {
	for _, body := range []string{
		`<html>blocked</html>`,
		`{"tag_name":"v9.9.9","assets":[{"name":"test.gz","browser_download_url":"https://github.com/MetaCubeX/mihomo/releases/download/v9.9.9/test.gz"}]}`,
		`{"tag_name":"v1.2.3","assets":[{"name":"test.gz","browser_download_url":"https://evil.example/test.gz"}]}`,
	} {
		f := testFetcher(t, func(Route, *http.Request) (int, string, error) { return 200, body, nil })
		if _, err := f.Release(context.Background(), "v1.2.3"); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}
