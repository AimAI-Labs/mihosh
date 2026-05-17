package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
)

func TestRenderGroupList(t *testing.T) {
	groups := map[string]model.Group{
		"Auto": {
			Name: "Auto",
			Type: "Selector",
			Now:  "HK",
			All:  []string{"HK", "US"},
		},
		"Global": {
			Name: "Global",
			Type: "Selector",
			Now:  "Direct",
			All:  []string{"Direct", "Auto"},
		},
	}
	orderedNames := []string{"Global", "Auto"}
	proxies := map[string]model.Proxy{
		"HK":     {History: []model.Delay{{Delay: 10}}},
		"US":     {History: []model.Delay{{Delay: 20}}},
		"Direct": {History: []model.Delay{{Delay: 5}}},
		"Auto":   {History: []model.Delay{{Delay: 30}}},
	}

	tests := []struct {
		name     string
		format   outputFormat
		contains []string
	}{
		{
			name:   "JSON format",
			format: outputFormatJSON,
			contains: []string{
				`"groups"`,
				`"name": "Auto"`,
				`"delay_ms": 10`,
			},
		},
		{
			name:   "Table format",
			format: outputFormatTable,
			contains: []string{
				"GROUP",
				"PROXY",
				"Auto",
				"HK",
			},
		},
	}

	t.Run("RespectsConfiguredOrder (Plain)", func(t *testing.T) {
		var out bytes.Buffer
		err := renderGroupList(&out, groups, orderedNames, proxies, outputFormatPlain)
		assert.NoError(t, err)

		output := out.String()
		globalIdx := strings.Index(output, "[Selector] Global")
		autoIdx := strings.Index(output, "[Selector] Auto")
		assert.NotEqual(t, -1, globalIdx, "missing Global group output")
		assert.NotEqual(t, -1, autoIdx, "missing Auto group output")
		assert.Less(t, globalIdx, autoIdx, "expected Global before Auto")
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := renderGroupList(&out, groups, orderedNames, proxies, tt.format)
			assert.NoError(t, err)

			output := out.String()
			for _, c := range tt.contains {
				assert.Contains(t, output, c)
			}
		})
	}
}
