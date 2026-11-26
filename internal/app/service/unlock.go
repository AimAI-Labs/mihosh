package service

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
)

// LoadUnlockResults loads cached streaming/service unlock check results for the given proxy and controller.
func LoadUnlockResults(proxyAddr string, client *api.Client) map[string]model.UnlockResult {
	return config.LoadUnlockResults(unlockResultsScope(proxyAddr, client))
}

// SaveUnlockResults persists streaming/service unlock check results for the given proxy and controller.
func SaveUnlockResults(proxyAddr string, client *api.Client, results map[string]model.UnlockResult) error {
	return config.SaveUnlockResults(unlockResultsScope(proxyAddr, client), results)
}

func unlockResultsScope(proxyAddr string, client *api.Client) string {
	var endpoint config.MihomoEndpoint
	if client != nil {
		endpoint = client.EndpointSnapshot()
	}
	return config.UnlockResultsScope(proxyAddr, endpoint)
}

const unlockTimeout = 15 * time.Second

type unlockCheck struct {
	name string
	run  func(unlockClient) model.UnlockResult
}

var unlockChecks = [...]unlockCheck{
	{"Bilibili (CN)", checkBilibili("https://api.bilibili.com/pgc/player/web/playurl?avid=82846771&ep_id=307247&qn=0&fnver=0&fnval=16&fourk=1&module=bangumi&otype=json")},
	{"Bilibili (HK/MO/TW)", checkBilibili("https://api.bilibili.com/pgc/player/web/playurl?avid=18281381&cid=29892777&ep_id=183799&qn=0&fnver=0&fnval=16&fourk=1&module=bangumi&otype=json")},
	{"ChatGPT Web", checkChatGPT},
	{"Claude", checkClaude},
	{"Gemini", checkGemini},
	{"Muse AI", checkMuse},
	{"YouTube Premium", checkYouTubePremium},
	{"Bahamut Anime", checkBahamut},
	{"Netflix", checkNetflix},
	{"Disney+", checkDisney},
	{"Prime Video", checkPrimeVideo},
	{"Spotify", checkSpotify},
	{"TikTok", checkTikTok},
}

func UnlockServiceNames() []string {
	names := make([]string, len(unlockChecks))
	for i, check := range unlockChecks {
		names[i] = check.name
	}
	return names
}

// CheckUnlock tests the effective egress for one service through Mihomo's proxy.
func CheckUnlock(ctx context.Context, proxyURL string, index int) model.UnlockResult {
	if index < 0 || index >= len(unlockChecks) {
		return model.UnlockResult{Status: model.UnlockFailed, CheckedAt: time.Now()}
	}
	client, err := newProxyProbeClient(proxyURL)
	if err != nil {
		return model.UnlockResult{Status: model.UnlockFailed, CheckedAt: time.Now()}
	}
	defer client.CloseIdleConnections()
	client.Jar, _ = cookiejar.New(nil)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		base := strings.TrimPrefix(via[0].URL.Hostname(), "www.")
		host := req.URL.Hostname()
		if len(via) >= 5 || req.URL.Scheme != "https" || req.URL.Port() != "" ||
			(host != base && !strings.HasSuffix(host, "."+base)) {
			return http.ErrUseLastResponse
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, unlockTimeout)
	defer cancel()
	result := unlockChecks[index].run(unlockClient{ctx, client})
	result.CheckedAt = time.Now()
	return result
}

type unlockClient struct {
	ctx  context.Context
	http *http.Client
}

type unlockResponse struct {
	code      int
	body      string
	url       *url.URL
	challenge bool
}

func (c unlockClient) get(endpoint string) unlockResponse {
	req, err := http.NewRequestWithContext(c.ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return unlockResponse{}
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	resp, err := c.http.Do(req)
	if err != nil {
		return unlockResponse{}
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return unlockResponse{}
	}
	return unlockResponse{code: resp.StatusCode, body: string(body), url: resp.Request.URL,
		challenge: resp.Header.Get("Cf-Mitigated") == "challenge"}
}

func (r unlockResponse) status() model.UnlockStatus {
	switch {
	case r.challenge || r.code == http.StatusTooManyRequests || (r.code >= 300 && r.code < 400):
		return model.UnlockInconclusive
	case r.code == http.StatusForbidden || r.code == http.StatusUnavailableForLegalReasons:
		return model.UnlockRestricted
	case r.code >= 200 && r.code < 300:
		return model.UnlockAvailable
	default:
		return model.UnlockFailed
	}
}

func unlockResult(status model.UnlockStatus, candidates ...string) model.UnlockResult {
	for _, candidate := range candidates {
		if code := regionCode(candidate); code != "" {
			return model.UnlockResult{Status: status, Region: code}
		}
	}
	return model.UnlockResult{Status: status}
}

func regionCode(value string) string {
	code := strings.ToUpper(strings.TrimSpace(value))
	if cut := strings.IndexAny(code, "-_"); cut >= 0 {
		code = code[:cut]
	}
	if len(code) < 2 || len(code) > 3 || code == "XX" || strings.IndexFunc(code, func(r rune) bool { return r < 'A' || r > 'Z' }) >= 0 {
		return ""
	}
	return code
}

func (r unlockResponse) field(key string) string {
	value := r.fieldValue(key)
	if len(value) == 0 || value[0] != '"' {
		return ""
	}
	text, _, _ := strings.Cut(value[1:], "\"")
	return text
}

func (r unlockResponse) fieldValue(key string) string {
	quoted := strconv.Quote(key)
	rest := r.body
	for {
		_, tail, ok := strings.Cut(rest, quoted)
		if !ok {
			return ""
		}
		trimmed := strings.TrimSpace(tail)
		if after, hasColon := strings.CutPrefix(trimmed, ":"); hasColon {
			return strings.TrimSpace(after)
		}
		rest = tail
	}
}

func (r unlockResponse) traceRegion() string {
	for line := range strings.SplitSeq(r.body, "\n") {
		if value, ok := strings.CutPrefix(line, "loc="); ok {
			return regionCode(value)
		}
	}
	return ""
}

func (r unlockResponse) contains(text ...string) bool {
	body := strings.ToLower(r.body)
	for _, value := range text {
		if strings.Contains(body, value) {
			return true
		}
	}
	return false
}
