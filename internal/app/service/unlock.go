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
)

type UnlockStatus string

const (
	UnlockAvailable    UnlockStatus = "available"
	UnlockRestricted   UnlockStatus = "restricted"
	UnlockOriginals    UnlockStatus = "originals"
	UnlockComingSoon   UnlockStatus = "coming_soon"
	UnlockInconclusive UnlockStatus = "inconclusive"
	UnlockFailed       UnlockStatus = "failed"
)

type UnlockResult struct {
	Status    UnlockStatus
	Region    string
	CheckedAt time.Time
}

const unlockTimeout = 15 * time.Second

type unlockCheck struct {
	name string
	run  func(unlockClient) UnlockResult
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
func CheckUnlock(ctx context.Context, proxyURL string, index int) UnlockResult {
	if index < 0 || index >= len(unlockChecks) {
		return UnlockResult{Status: UnlockFailed, CheckedAt: time.Now()}
	}
	client, err := newProxyProbeClient(proxyURL)
	if err != nil {
		return UnlockResult{Status: UnlockFailed, CheckedAt: time.Now()}
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
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return unlockResponse{}
	}
	return unlockResponse{code: resp.StatusCode, body: string(body), url: resp.Request.URL,
		challenge: resp.Header.Get("Cf-Mitigated") == "challenge"}
}

func (r unlockResponse) status() UnlockStatus {
	switch {
	case r.challenge || r.code == http.StatusTooManyRequests || (r.code >= 300 && r.code < 400):
		return UnlockInconclusive
	case r.code == http.StatusForbidden || r.code == http.StatusUnavailableForLegalReasons:
		return UnlockRestricted
	case r.code >= 200 && r.code < 300:
		return UnlockAvailable
	default:
		return UnlockFailed
	}
}

func unlockResult(status UnlockStatus, candidates ...string) UnlockResult {
	for _, candidate := range candidates {
		if code := regionCode(candidate); code != "" {
			return UnlockResult{Status: status, Region: code}
		}
	}
	return UnlockResult{Status: status}
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
	_, tail, ok := strings.Cut(r.body, strconv.Quote(key))
	if !ok {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(tail), ":"))
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
