package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/jeandeaual/go-locale"
)

//go:embed locales/*.json
var localesFS embed.FS

var (
	isChinese bool
	dict      map[string]string
	fallback  map[string]string
	mutex     sync.RWMutex
)

// Init initializes the i18n system by detecting system locale and loading the correct JSON.
func Init() {
	mutex.Lock()
	defer mutex.Unlock()

	dict = make(map[string]string)
	fallback = make(map[string]string)

	// Load fallback (en-US)
	enBytes, err := localesFS.ReadFile("locales/en-US.json")
	if err == nil {
		_ = json.Unmarshal(enBytes, &fallback)
	}

	// Load Chinese (zh-CN)
	zhBytes, err := localesFS.ReadFile("locales/zh-CN.json")
	if err == nil {
		_ = json.Unmarshal(zhBytes, &dict)
	}

	// Detect system language
	localesList, err := locale.GetLocales()
	if err == nil && len(localesList) > 0 {
		if strings.HasPrefix(strings.ToLower(localesList[0]), "zh") {
			isChinese = true
		}
	}
}

// SetLanguageOverride overrides the auto-detected language
func SetLanguageOverride(lang string) {
	mutex.Lock()
	defer mutex.Unlock()
	if lang == "zh-CN" {
		isChinese = true
	} else if lang == "en-US" {
		isChinese = false
	} else if lang == "auto" || lang == "" {
		// Re-detect system language
		isChinese = false
		localesList, err := locale.GetLocales()
		if err == nil && len(localesList) > 0 {
			if strings.HasPrefix(strings.ToLower(localesList[0]), "zh") {
				isChinese = true
			}
		}
	}
}

// T translates a key. If the system is not Chinese, it uses the fallback dictionary.
// If the key is not found in the appropriate dictionary, it returns the key itself as a fallback.
func T(key string) string {
	mutex.RLock()
	defer mutex.RUnlock()

	if isChinese {
		if val, ok := dict[key]; ok {
			return val
		}
		// If key not in Chinese dict, try fallback
		if val, ok := fallback[key]; ok {
			return val
		}
	} else {
		if val, ok := fallback[key]; ok {
			return val
		}
	}

	// If missing, return key
	return key
}

// IsChinese returns whether the current system language is detected as Chinese.
func IsChinese() bool {
	mutex.RLock()
	defer mutex.RUnlock()
	return isChinese
}

// Tf translates a key and formats it with arguments.
func Tf(key string, args ...interface{}) string {
	format := T(key)
	return fmt.Sprintf(format, args...)
}
