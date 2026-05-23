package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/stretchr/testify/require"
)

func TestGetMemoryDecodesFirstObjectFromOpenStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/memory", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"inuse":1024,"oslimit":2048}` + "\n"))
		require.NoError(t, err)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	client := NewClient(&config.Config{
		APIAddress: server.URL,
		Timeout:    100,
	})

	start := time.Now()
	mem, err := client.GetMemory()

	require.NoError(t, err)
	require.Less(t, time.Since(start), 100*time.Millisecond)
	require.Equal(t, int64(1024), mem.Inuse)
	require.Equal(t, int64(2048), mem.OSLimit)
}
