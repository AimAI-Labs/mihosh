package service

import (
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

func checkBilibili(endpoint string) func(unlockClient) UnlockResult {
	return func(c unlockClient) UnlockResult {
		r := c.get(endpoint)
		if status := r.status(); status != UnlockAvailable {
			return unlockResult(status)
		}
		var response struct{ Code *int }
		if json.Unmarshal([]byte(r.body), &response) != nil || response.Code == nil {
			return unlockResult(UnlockInconclusive)
		}
		switch *response.Code {
		case 0:
			return unlockResult(UnlockAvailable)
		case -10403:
			return unlockResult(UnlockRestricted)
		default:
			return unlockResult(UnlockInconclusive)
		}
	}
}

func checkChatGPT(c unlockClient) UnlockResult {
	r := c.get("https://api.openai.com/compliance/cookie_requirements")
	status := r.status()
	if r.contains("unsupported_country", "unsupported_region") {
		status = UnlockRestricted
	} else if status == UnlockAvailable && !json.Valid([]byte(r.body)) {
		status = UnlockInconclusive
	}
	if status == UnlockFailed {
		return unlockResult(status)
	}
	trace := c.get("https://chatgpt.com/cdn-cgi/trace")
	return unlockResult(status, trace.traceRegion())
}

func checkClaude(c unlockClient) UnlockResult {
	r := c.get("https://claude.ai/cdn-cgi/trace")
	status := r.status()
	region := r.traceRegion()
	if status == UnlockAvailable {
		blocked := []string{"AF", "BY", "CN", "CU", "HK", "IR", "KP", "MO", "RU", "SY"}
		switch {
		case region == "":
			status = UnlockInconclusive
		case slices.Contains(blocked, region):
			status = UnlockRestricted
		}
	}
	return unlockResult(status, region)
}

func checkGemini(c unlockClient) UnlockResult {
	r := c.get("https://gemini.google.com/")
	if status := r.status(); status != UnlockAvailable {
		return unlockResult(status)
	}
	_, segment, _ := strings.Cut(r.body, ",2,1,200,\"")
	region, _, _ := strings.Cut(segment, "\"")
	region = regionCode(region)
	blocked := []string{"CHN", "RUS", "BLR", "CUB", "IRN", "PRK", "SYR", "HKG", "MAC"}
	switch {
	case region == "":
		return unlockResult(UnlockInconclusive)
	case slices.Contains(blocked, region):
		return unlockResult(UnlockRestricted, region)
	default:
		return unlockResult(UnlockAvailable, region)
	}
}

func checkMuse(c unlockClient) UnlockResult {
	r := c.get("https://muse.ai/")
	if (r.url != nil && r.url.Hostname() == "auth.muse.ai") ||
		r.contains("static.facebook.com/images/logos", "sorry, something went wrong") {
		return unlockResult(UnlockInconclusive)
	}
	return unlockResult(r.status())
}

func checkYouTubePremium(c unlockClient) UnlockResult {
	r := c.get("https://www.youtube.com/premium?hl=en")
	status := r.status()
	if r.contains("premium is not available in your country", "premium is not available in your region") {
		status = UnlockRestricted
	} else if status == UnlockAvailable && !r.contains("ad-free", "spunlimited") {
		status = UnlockInconclusive
	}
	return unlockResult(status, r.field("GL"), r.field("countryCode"), r.field("country_code"))
}

func checkBahamut(c unlockClient) UnlockResult {
	device := c.get("https://ani.gamer.com.tw/ajax/getdeviceid.php")
	if status := device.status(); status != UnlockAvailable {
		return unlockResult(status)
	}
	id := device.field("deviceid")
	if id == "" {
		return unlockResult(UnlockInconclusive)
	}
	r := c.get("https://ani.gamer.com.tw/ajax/token.php?sn=37783&adID=89422&device=" + url.QueryEscape(id))
	if status := r.status(); status != UnlockAvailable {
		return unlockResult(status)
	}
	if r.fieldValue("animeSn") != "" {
		home := c.get("https://ani.gamer.com.tw/")
		_, country, _ := strings.Cut(home.body, "data-geo=\"")
		region, _, _ := strings.Cut(country, "\"")
		return unlockResult(UnlockAvailable, region)
	}
	if r.contains("地區", "区域", "region") {
		return unlockResult(UnlockRestricted)
	}
	return unlockResult(UnlockInconclusive)
}

func checkNetflix(c unlockClient) UnlockResult {
	first := c.get("https://www.netflix.com/title/81280792")
	second := c.get("https://www.netflix.com/title/70143836")
	for _, r := range []unlockResponse{first, second} {
		if r.status() == UnlockAvailable && r.url != nil && strings.Contains(r.url.Path, "/title/") {
			if region := netflixRegion(r); region != "" {
				return unlockResult(UnlockAvailable, region)
			}
			locale := c.get("https://www.netflix.com/title/80018499")
			return unlockResult(UnlockAvailable, netflixRegion(locale))
		}
	}
	switch {
	case first.code == http.StatusNotFound && second.code == http.StatusNotFound:
		return unlockResult(UnlockOriginals)
	case first.status() == UnlockRestricted || second.status() == UnlockRestricted:
		return unlockResult(UnlockRestricted)
	case first.status() == UnlockInconclusive || second.status() == UnlockInconclusive:
		return unlockResult(UnlockInconclusive)
	case first.status() == UnlockAvailable || second.status() == UnlockAvailable:
		return unlockResult(UnlockInconclusive)
	default:
		return unlockResult(UnlockFailed)
	}
}

func netflixRegion(r unlockResponse) string {
	if r.url != nil {
		locale, _, _ := strings.Cut(strings.TrimPrefix(r.url.Path, "/"), "/")
		return regionCode(locale)
	}
	return ""
}

func checkDisney(c unlockClient) UnlockResult {
	r := c.get("https://www.disneyplus.com/")
	region := r.field("region")
	if regionCode(region) == "" {
		region = r.field("countryCode")
	}
	status := r.status()
	switch {
	case r.url != nil && (strings.Contains(r.url.Path, "/unavailable") || strings.Contains(r.url.Path, "/preview")):
		status = UnlockRestricted
	case r.contains("not available in your region", "not available in your country", "forbidden-location"):
		status = UnlockRestricted
	case status == UnlockAvailable:
		switch {
		case strings.HasPrefix(r.fieldValue("inSupportedLocation"), "false"):
			status = UnlockComingSoon
		case regionCode(region) == "" && !strings.HasPrefix(r.fieldValue("inSupportedLocation"), "true"):
			status = UnlockInconclusive
		}
	}
	return unlockResult(status, region)
}

func checkPrimeVideo(c unlockClient) UnlockResult {
	r := c.get("https://www.primevideo.com/")
	status := r.status()
	region := r.field("currentTerritory")
	if strings.HasPrefix(r.fieldValue("isServiceRestricted"), "true") ||
		r.contains("service is restricted", "not available in your location") {
		status = UnlockRestricted
	} else if status == UnlockAvailable && regionCode(region) == "" {
		status = UnlockInconclusive
	}
	return unlockResult(status, region)
}

func checkSpotify(c unlockClient) UnlockResult {
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
		status = UnlockRestricted
	} else if status == UnlockAvailable && region == "" && !json.Valid([]byte(r.body)) {
		status = UnlockInconclusive
	}
	return unlockResult(status, region)
}

func checkTikTok(c unlockClient) UnlockResult {
	r := c.get("https://www.tiktok.com/")
	status := r.status()
	region := r.field("region")
	if r.contains("tiktok is not available", "not available in your region", "access denied") {
		status = UnlockRestricted
	} else if status == UnlockAvailable && regionCode(region) == "" {
		trace := c.get("https://www.tiktok.com/cdn-cgi/trace")
		region = trace.traceRegion()
		if region == "" {
			status = UnlockInconclusive
		}
	}
	return unlockResult(status, region)
}
