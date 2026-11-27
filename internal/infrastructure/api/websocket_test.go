package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func TestWSClient_StartStopAndHandlers(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		switch {
		case strings.Contains(r.URL.Path, "/memory"):
			data := MemoryData{Inuse: 1024, OSLimit: 2048}
			msg, _ := json.Marshal(data)
			_ = conn.WriteMessage(websocket.TextMessage, msg)
		case strings.Contains(r.URL.Path, "/traffic"):
			data := TrafficData{Up: 100, Down: 200}
			msg, _ := json.Marshal(data)
			_ = conn.WriteMessage(websocket.TextMessage, msg)
		case strings.Contains(r.URL.Path, "/connections"):
			data := ConnectionsData{DownloadTotal: 500}
			msg, _ := json.Marshal(data)
			_ = conn.WriteMessage(websocket.TextMessage, msg)
		case strings.Contains(r.URL.Path, "/logs"):
			data := LogData{Type: "info", Payload: "test log"}
			msg, _ := json.Marshal(data)
			_ = conn.WriteMessage(websocket.TextMessage, msg)
		}

		// Keep connection open until client closes it
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}))
	defer ts.Close()

	client := NewWSClient(ts.URL, "")
	var wg sync.WaitGroup
	wg.Add(4)

	client.SetMemoryHandler(func(d MemoryData) {
		if d.Inuse == 1024 {
			wg.Done()
		}
	})
	client.SetTrafficHandler(func(d TrafficData) {
		if d.Up == 100 {
			wg.Done()
		}
	})
	client.SetConnectionsHandler(func(d ConnectionsData) {
		if d.DownloadTotal == 500 {
			wg.Done()
		}
	})
	client.SetLogsHandler(func(d LogData) {
		if d.Payload == "test log" {
			wg.Done()
		}
	})

	err := client.Start(context.Background())
	if err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(3 * time.Second):
		t.Fatal("Timeout waiting for handlers to be called")
	}

	if !client.IsRunning() {
		t.Errorf("Expected IsRunning to be true")
	}

	client.Stop()

	if client.IsRunning() {
		t.Errorf("Expected IsRunning to be false after Stop()")
	}
}

func TestWSClient_Reconnection(t *testing.T) {
	var attempts, active atomic.Int32
	closed := make(chan error, 2)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		if r.URL.Path == "/memory" {
			if active.Add(1) != 1 {
				t.Error("memory endpoint had overlapping connections")
			}
			defer active.Add(-1)
			if attempts.Add(1) <= 2 {
				_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "reconnect"))
				_ = conn.UnderlyingConn().SetReadDeadline(time.Now().Add(3 * time.Second))
				// The server leaves TCP open after the close frame. Only the client
				// can end this read, including if it sent a close-frame reply.
				_, err := io.Copy(io.Discard, conn.UnderlyingConn())
				closed <- err
				return
			}
			_ = conn.WriteJSON(MemoryData{Inuse: 123})
		}
		_, _, _ = conn.ReadMessage()
	}))
	defer ts.Close()

	client := NewWSClient(ts.URL, "")
	received := make(chan MemoryData, 1)
	client.SetMemoryHandler(func(d MemoryData) { received <- d })
	if err := client.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer client.Stop()
	for range 2 {
		if err := awaitWS(t, closed); err != nil {
			t.Fatalf("client failed to close the socket after a read error: %v", err)
		}
	}
	if got := awaitWS(t, received); got.Inuse != 123 || attempts.Load() != 3 {
		t.Fatalf("unexpected reconnection: %+v, attempts %d", got, attempts.Load())
	}
}

func TestWSClientStopCancelsBlockedHandshake(t *testing.T) {
	started, released := make(chan struct{}, 4), make(chan struct{}, 4)
	unblock := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		select {
		case <-r.Context().Done():
			released <- struct{}{}
		case <-unblock:
		}
	}))
	defer ts.Close()
	defer close(unblock)
	client := NewWSClient(ts.URL, "")
	if err := client.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	for range 4 {
		awaitWS(t, started)
	}
	done := make(chan struct{})
	go func() { client.Stop(); close(done) }()
	awaitWS(t, done)
	for range 4 {
		awaitWS(t, released)
	}
}

func TestWSClientSwitchThenParentCancellationClosesReadLoops(t *testing.T) {
	newTarget := func(token string, value int64) (*httptest.Server, <-chan string, <-chan string) {
		connected, closed := make(chan string, 4), make(chan string, 4)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("token") != token {
				t.Errorf("wrong token for %s", r.URL.Path)
			}
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer func() { _ = conn.Close() }()
			connected <- r.URL.Path
			if r.URL.Path == "/memory" {
				_ = conn.WriteJSON(MemoryData{Inuse: value})
			}
			_, _, _ = conn.ReadMessage()
			closed <- r.URL.Path
		}))
		t.Cleanup(server.Close)
		return server, connected, closed
	}
	old, oldConnected, oldClosed := newTarget("old", 1)
	next, nextConnected, nextClosed := newTarget("new", 2)
	client := NewWSClient(old.URL, "old")
	received := make(chan MemoryData, 4)
	client.SetMemoryHandler(func(d MemoryData) { received <- d })
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer client.Stop()
	checkPaths := func(events <-chan string) {
		t.Helper()
		paths := map[string]bool{}
		for range 4 {
			paths[awaitWS(t, events)] = true
		}
		if len(paths) != 4 {
			t.Fatalf("expected one stream per endpoint, got %v", paths)
		}
	}
	checkPaths(oldConnected)
	if data := awaitWS(t, received); data.Inuse != 1 || data.Generation != 0 {
		t.Fatalf("old snapshot: %+v", data)
	}
	client.UpdateEndpoint(next.URL, "new")
	checkPaths(oldClosed)
	checkPaths(nextConnected)
	if data := awaitWS(t, received); data.Inuse != 2 || data.Generation != client.Generation() || data.Generation == 0 {
		t.Fatalf("new snapshot: %+v", data)
	}
	cancel()
	checkPaths(nextClosed)
	client.Stop()
	if client.IsRunning() {
		t.Fatal("client still running after cancellation")
	}
}

func awaitWS[T any](t *testing.T, events <-chan T) T {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(4 * time.Second):
		t.Fatal("timeout waiting for websocket event")
		var zero T
		return zero
	}
}
