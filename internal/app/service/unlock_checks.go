package service

import (
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
)

func checkBilibili(endpoint string) func(unlockClient) model.UnlockResult {
	return func(c unlockClient) model.UnlockResult {
		r := c.get(endpoint)
		if status := r.status(); status != model.UnlockAvailable {
			return unlockResult(status)
		}
		var response struct{ Code *int }
		if json.Unmarshal([]byte(r.body), &response) != nil || response.Code == nil {
			return unlockResult(model.UnlockInconclusive)
		}
		switch *response.Code {
		case 0:
			return unlockResult(model.UnlockAvailable)
		case -10403:
			return unlockResult(model.UnlockRestricted)
		default:
			return unlockResult(model.UnlockInconclusive)
		}
	}
}

func checkChatGPT(c unlockClient) model.UnlockResult {
	r := c.get("https://api.openai.com/compliance/cookie_requirements")
	status := r.status()
	if r.contains("unsupported_country", "unsupported_region") {
		status = model.UnlockRestricted
	} else if status == model.UnlockAvailable && !json.Valid([]byte(r.body)) {
		status = model.UnlockInconclusive
	}
	if status == model.UnlockFailed {
		return unlockResult(status)
	}
	trace := c.get("https://chatgpt.com/cdn-cgi/trace")
	return unlockResult(status, trace.traceRegion())
}

func checkClaude(c unlockClient) model.UnlockResult {
	r := c.get("https://claude.ai/cdn-cgi/trace")
	status := r.status()
	region := r.traceRegion()
	if status == model.UnlockAvailable {
		blocked := []string{"AF", "BY", "CN", "CU", "HK", "IR", "KP", "MO", "RU", "SY"}
		switch {
		case region == "":
			status = model.UnlockInconclusive
		case slices.Contains(blocked, region):
			status = model.UnlockRestricted
		}
	}
	return unlockResult(status, region)
}

func checkGemini(c unlockClient) model.UnlockResult {
	r := c.get("https://gemini.google.com/")
	if status := r.status(); status != model.UnlockAvailable {
		return unlockResult(status)
	}
	_, segment, _ := strings.Cut(r.body, ",2,1,200,\"")
	region, _, _ := strings.Cut(segment, "\"")
	region = regionCode(region)
	blocked := []string{"CHN", "RUS", "BLR", "CUB", "IRN", "PRK", "SYR", "HKG", "MAC"}
	switch {
	case region == "":
		return unlockResult(model.UnlockInconclusive)
	case slices.Contains(blocked, region):
		return unlockResult(model.UnlockRestricted, region)
	default:
		return unlockResult(model.UnlockAvailable, region)
	}
}

func checkMuse(c unlockClient) model.UnlockResult {
	r := c.get("https://muse.ai/")
	if (r.url != nil && r.url.Hostname() == "auth.muse.ai") ||
		r.contains("static.facebook.com/images/logos", "sorry, something went wrong") {
		return unlockResult(model.UnlockInconclusive)
	}
	return unlockResult(r.status())
}

func checkYouTubePremium(c unlockClient) model.UnlockResult {
	r := c.get("https://www.youtube.com/premium?hl=en")
	status := r.status()
	if r.contains("premium is not available in your country", "premium is not available in your region") {
		status = model.UnlockRestricted
	} else if status == model.UnlockAvailable && !r.contains("ad-free", "spunlimited") {
		status = model.UnlockInconclusive
	}
	return unlockResult(status, r.field("GL"), r.field("countryCode"), r.field("country_code"))
}

func checkBahamut(c unlockClient) model.UnlockResult {
	device := c.get("https://ani.gamer.com.tw/ajax/getdeviceid.php")
	if status := device.status(); status != model.UnlockAvailable {
		return unlockResult(status)
	}
	id := device.field("deviceid")
	if id == "" {
		return unlockResult(model.UnlockInconclusive)
	}
	r := c.get("https://ani.gamer.com.tw/ajax/token.php?sn=37783&adID=89422&device=" + url.QueryEscape(id))
	if status := r.status(); status != model.UnlockAvailable {
		return unlockResult(status)
	}
	if r.fieldValue("animeSn") != "" {
		home := c.get("https://ani.gamer.com.tw/")
		_, country, _ := strings.Cut(home.body, "data-geo=\"")
		region, _, _ := strings.Cut(country, "\"")
		return unlockResult(model.UnlockAvailable, region)
	}
	if r.contains("地區", "区域", "region") {
		return unlockResult(model.UnlockRestricted)
	}
	return unlockResult(model.UnlockInconclusive)
}

func checkNetflix(c unlockClient) model.UnlockResult {
	first := c.get("https://www.netflix.com/title/81280792")
	second := c.get("https://www.netflix.com/title/70143836")
	for _, r := range []unlockResponse{first, second} {
		if r.status() == model.UnlockAvailable && r.url != nil && strings.Contains(r.url.Path, "/title/") {
			if region := netflixRegion(r); region != "" {
				return unlockResult(model.UnlockAvailable, region)
			}
			locale := c.get("https://www.netflix.com/title/80018499")
			return unlockResult(model.UnlockAvailable, netflixRegion(locale))
		}
	}
	switch {
	case first.code == http.StatusNotFound && second.code == http.StatusNotFound:
		return unlockResult(model.UnlockOriginals)
	case first.status() == model.UnlockRestricted || second.status() == model.UnlockRestricted:
		return unlockResult(model.UnlockRestricted)
	case first.status() == model.UnlockInconclusive || second.status() == model.UnlockInconclusive:
		return unlockResult(model.UnlockInconclusive)
	case first.status() == model.UnlockAvailable || second.status() == model.UnlockAvailable:
		return unlockResult(model.UnlockInconclusive)
	default:
		return unlockResult(model.UnlockFailed)
	}
}

func netflixRegion(r unlockResponse) string {
	if r.url != nil {
		locale, _, _ := strings.Cut(strings.TrimPrefix(r.url.Path, "/"), "/")
		return regionCode(locale)
	}
	return ""
}

func checkDisney(c unlockClient) model.UnlockResult {
	r := c.get("https://www.disneyplus.com/")
	region := r.field("region")
	if regionCode(region) == "" {
		region = r.field("countryCode")
	}
	status := r.status()
	switch {
	case r.url != nil && (strings.Contains(r.url.Path, "/unavailable") || strings.Contains(r.url.Path, "/preview")):
		status = model.UnlockRestricted
	case r.contains("not available in your region", "not available in your country", "forbidden-location"):
		status = model.UnlockRestricted
	case status == model.UnlockAvailable:
		switch {
		case strings.HasPrefix(r.fieldValue("inSupportedLocation"), "false"):
			status = model.UnlockComingSoon
		case regionCode(region) == "" && !strings.HasPrefix(r.fieldValue("inSupportedLocation"), "true"):
			status = model.UnlockInconclusive
		}
	}
	return unlockResult(status, region)
}

func checkPrimeVideo(c unlockClient) model.UnlockResult {
	r := c.get("https://www.primevideo.com/")
	status := r.status()
	region := r.field("currentTerritory")
	if strings.HasPrefix(r.fieldValue("isServiceRestricted"), "true") ||
		r.contains("service is restricted", "not available in your location") {
		status = model.UnlockRestricted
	} else if status == model.UnlockAvailable && regionCode(region) == "" {
		status = model.UnlockInconclusive
	}
	return unlockResult(status, region)
}

func checkSpotify(c unlockClient) model.UnlockResult {
	r := c.get("https://www.spotify.com/api/content/v1/country-selector?platform=web&format=json")
	status := r.status()
	region := r.field("countryCode")
	if region == "" && r.url != nil {
		locale, _, _ := strings.Cut(strings.TrimPrefix(r.url.Path, "/"), "/")
		if code := regionCode(locale); len(code) == 2 {
			region = code
		}
	}
	if r.contains("not available in your country") {
		status = model.UnlockRestricted
	} else if status == model.UnlockAvailable && region == "" && !json.Valid([]byte(r.body)) {
		status = model.UnlockInconclusive
	}
	return unlockResult(status, region)
}

func checkTikTok(c unlockClient) model.UnlockResult {
	r := c.get("https://www.tiktok.com/")
	status := r.status()
	region := r.field("region")
	if r.contains("tiktok is not available", "not available in your region", "access denied") {
		status = model.UnlockRestricted
	} else if status == model.UnlockAvailable && regionCode(region) == "" {
		trace := c.get("https://www.tiktok.com/cdn-cgi/trace")
		region = trace.traceRegion()
		if region == "" {
			status = model.UnlockInconclusive
		}
	}
	return unlockResult(status, region)
}
