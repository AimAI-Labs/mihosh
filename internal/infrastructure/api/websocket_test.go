package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
		defer conn.Close()

		switch {
		case strings.Contains(r.URL.Path, "/memory"):
			data := MemoryData{Inuse: 1024, OSLimit: 2048}
			msg, _ := json.Marshal(data)
			conn.WriteMessage(websocket.TextMessage, msg)
		case strings.Contains(r.URL.Path, "/traffic"):
			data := TrafficData{Up: 100, Down: 200}
			msg, _ := json.Marshal(data)
			conn.WriteMessage(websocket.TextMessage, msg)
		case strings.Contains(r.URL.Path, "/connections"):
			data := ConnectionsData{DownloadTotal: 500}
			msg, _ := json.Marshal(data)
			conn.WriteMessage(websocket.TextMessage, msg)
		case strings.Contains(r.URL.Path, "/logs"):
			data := LogData{Type: "info", Payload: "test log"}
			msg, _ := json.Marshal(data)
			conn.WriteMessage(websocket.TextMessage, msg)
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
	var connCount int
	var mu sync.Mutex

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		mu.Lock()
		connCount++
		mu.Unlock()

		// Immediately close to trigger reconnection logic
		conn.Close()
	}))
	defer ts.Close()

	client := NewWSClient(ts.URL, "")
	// We only start memory stream manually to test reconnection simply
	go connectStream(client, "memory", func(d MemoryData) {})
	
	// Need to manually set isRunning for test to avoid immediate exit in connectStream loop
	client.runningMu.Lock()
	client.isRunning = true
	client.ctx, client.cancel = context.WithCancel(context.Background())
	client.runningMu.Unlock()

	time.Sleep(2500 * time.Millisecond) // enough time for initial connection + 1s delay + second connection

	client.Stop()

	mu.Lock()
	defer mu.Unlock()
	if connCount < 2 {
		t.Errorf("Expected at least 2 connections (reconnection), got %d", connCount)
	}
}

func TestWSClient_UpdateEndpoint(t *testing.T) {
	ts1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}))
	defer ts1.Close()

	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}))
	defer ts2.Close()

	client := NewWSClient(ts1.URL, "secret1")
	client.Start(context.Background())
	
	// give it a moment to connect
	time.Sleep(100 * time.Millisecond)

	// Update endpoint
	client.UpdateEndpoint(ts2.URL, "secret2")
	
	client.mu.RLock()
	baseURL := client.baseURL
	secret := client.secret
	client.mu.RUnlock()

	if baseURL != ts2.URL {
		t.Errorf("Expected baseURL %s, got %s", ts2.URL, baseURL)
	}
	if secret != "secret2" {
		t.Errorf("Expected secret %s, got %s", "secret2", secret)
	}
	
	client.Stop()
}
