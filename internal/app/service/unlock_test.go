package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
)

type unlockFixture func(*http.Request) (*http.Response, error)

func (f unlockFixture) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestUnlockChecksInterpretServiceResponses(t *testing.T) {
	tests := []struct {
		name   string
		reply  func(*http.Request) (int, string)
		status model.UnlockStatus
		region string
	}{
		{"Bilibili (CN)", unlockReply(200, "{\"code\":0}"), model.UnlockAvailable, ""},
		{"Bilibili (HK/MO/TW)", unlockReply(200, "{\"code\":-10403}"), model.UnlockRestricted, ""},
		{"ChatGPT Web", func(req *http.Request) (int, string) {
			if req.URL.Host == "api.openai.com" {
				return 403, "{\"error\":{\"code\":\"unsupported_country\"}}"
			}
			return 200, "ip=1.2.3.4\nloc=HK\n"
		}, model.UnlockRestricted, "HK"},
		{"Claude", unlockReply(200, "loc=HK\n"), model.UnlockRestricted, "HK"},
		{"Gemini", unlockReply(200, "config:[1,2,1,200,\"JPN\",1]"), model.UnlockAvailable, "JPN"},
		{"Muse AI", unlockReply(200, "Sorry, something went wrong"), model.UnlockInconclusive, ""},
		{"YouTube Premium", unlockReply(200, "\"GL\": \"CA\", Premium is not available in your country"), model.UnlockRestricted, "CA"},
		{"Bahamut Anime", func(req *http.Request) (int, string) {
			if req.URL.Path == "/ajax/getdeviceid.php" {
				return 200, "{\"deviceid\":\"sample+id\"}"
			}
			if req.URL.Path == "/" {
				return 200, "<html data-geo=\"TW\">"
			}
			if req.URL.Query().Get("device") != "sample+id" {
				t.Errorf("Bahamut request lost the device ID: %s", req.URL)
			}
			return 200, "{\"animeSn\":37783}"
		}, model.UnlockAvailable, "TW"},
		{"Netflix", unlockReply(404, ""), model.UnlockOriginals, ""},
		{"Disney+", unlockReply(200, "{\"countryCode\":\"TW\",\"inSupportedLocation\":false}"), model.UnlockComingSoon, "TW"},
		{"Prime Video", unlockReply(200, "{\"isServiceRestricted\":false,\"currentTerritory\":\"GB\"}"), model.UnlockAvailable, "GB"},
		{"Spotify", unlockReply(200, "{\"countryCode\": \"DE\"}"), model.UnlockAvailable, "DE"},
		{"TikTok", unlockReply(200, "{\"region\":\"gb\"}"), model.UnlockAvailable, "GB"},
	}
	if len(tests) != len(unlockChecks) {
		t.Fatal("a service is missing a response fixture")
	}
	for index, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			check := unlockChecks[index]
			if check.name != tc.name {
				t.Fatalf("fixture for %q does not match catalog entry %q", tc.name, check.name)
			}
			client := &http.Client{Transport: unlockFixture(func(req *http.Request) (*http.Response, error) {
				code, body := tc.reply(req)
				return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
			})}
			got := check.run(unlockClient{t.Context(), client})
			if got.Status != tc.status || got.Region != tc.region {
				t.Fatalf("got %s %q, want %s %q", got.Status, got.Region, tc.status, tc.region)
			}
		})
	}
}

func unlockReply(code int, body string) func(*http.Request) (int, string) {
	return func(*http.Request) (int, string) { return code, body }
}

func TestUnlockWithoutProxyFails(t *testing.T) {
	result := CheckUnlock(context.Background(), "", 0)
	if result.Status != model.UnlockFailed || result.CheckedAt.IsZero() {
		t.Fatalf("got %+v", result)
	}
}
