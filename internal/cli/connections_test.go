package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/AimAI-Labs/mihosh/internal/domain/model"
)

func TestRenderConnections(t *testing.T) {
	resp := model.ConnectionsResponse{
		UploadTotal:   1024,
		DownloadTotal: 2048,
		Connections: []model.Connection{
			{
				Metadata: model.Metadata{
					SourceIP:        "10.0.0.1",
					SourcePort:      "1234",
					DestinationIP:   "1.1.1.1",
					DestinationPort: "443",
				},
				Chains: []string{"Proxy", "HK"},
			},
		},
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
				`"active_connections": 1`,
				`"source_ip": "10.0.0.1"`,
			},
		},
		{
			name:   "Table format",
			format: outputFormatTable,
			contains: []string{
				"SOURCE",
				"DESTINATION",
				"10.0.0.1:1234",
				"1.1.1.1:443",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := renderConnections(&out, resp, tt.format)
			assert.NoError(t, err)

			output := out.String()
			for _, c := range tt.contains {
				assert.Contains(t, output, c)
			}
		})
	}
}
