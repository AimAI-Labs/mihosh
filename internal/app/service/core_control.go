package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
)

var ErrNoCoreOwner = errors.New("no live Mihosh core owner")

type coreOwner struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}
type coreControlRequest struct {
	Action  string `json:"action"`
	Version string `json:"version,omitempty"`
}

type coreControlResponse struct {
	Result       string `json:"result"`
	BackupName   string `json:"backup_name,omitempty"`
	MergeWarning string `json:"merge_warning,omitempty"`
}

// startControl runs only in a session that successfully started its own child. It
// provides local lifecycle coordination, not an API for managing arbitrary processes.
// The caller holds s.mu. The bearer token is unrelated to the Mihomo API secret.
func (s *CoreService) startControl(root string) error {
	if s.control != nil {
		data, err := json.Marshal(s.controlOwner)
		if err != nil {
			return err
		}
		return privateWrite(filepath.Join(root, "runtime", "owner.json"), data)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	var secret [32]byte
	if _, err = rand.Read(secret[:]); err != nil {
		_ = listener.Close()
		return err
	}
	owner := coreOwner{"http://" + listener.Addr().String(), hex.EncodeToString(secret[:])}
	server := &http.Server{ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 180 * time.Second, IdleTimeout: 5 * time.Second, MaxHeaderBytes: 4096}
	slots := make(chan struct{}, 1)
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost || r.URL.Path != "/control" || r.Header.Get("Origin") != "" {
			http.Error(w, "invalid local control request", 400)
			return
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(token), []byte(owner.Token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			http.Error(w, "another local control operation is active", http.StatusConflict)
			return
		}
		var request coreControlRequest
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&request) != nil {
			http.Error(w, "invalid local control request", 400)
			return
		}
		var err error
		var activation ActivateResult
		switch request.Action {
		case "status":
			_, err = s.Status()
		case "stop":
			err = s.StopContext(r.Context())
		case "restart":
			err = s.Restart(r.Context())
		case "upgrade":
			err = s.UpgradeVersion(r.Context(), request.Version)
		case "activate":
			err = s.ActivateVersion(r.Context(), request.Version)
		case "rollback":
			err = s.ActivateVersion(r.Context(), "")
		case "profile":
			activation, err = s.ApplyProfile(r.Context(), request.Version, false)
		case "system-proxy", "tun":
			if request.Version != "on" && request.Version != "off" {
				http.Error(w, "invalid network setting", 400)
				return
			}
			if request.Action == "tun" {
				err = s.SetTUN(r.Context(), request.Version == "on")
			} else {
				err = s.SetSystemProxy(r.Context(), request.Version == "on")
			}
		default:
			http.Error(w, "unsupported local control action", 400)
			return
		}
		if err != nil {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		s.notifyOwner(request.Action)
		response := coreControlResponse{Result: "ok", BackupName: activation.BackupName}
		if activation.MergeErr != nil {
			response.MergeWarning = activation.MergeErr.Error()
		}
		_ = json.NewEncoder(w).Encode(response)
	})
	data, _ := json.Marshal(owner)
	if err = privateWrite(filepath.Join(root, "runtime", "owner.json"), data); err != nil {
		_ = listener.Close()
		return err
	}
	s.control = server
	s.controlOwner = owner
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		_ = server.Close()
		removeOwner(root, owner.Token)
	}
	s.controlCleanup = cleanup
	context.AfterFunc(s.lifetime, cleanup)
	go func() { _ = server.Serve(listener) }()
	return nil
}
func removeOwner(root, token string) {
	path := filepath.Join(root, "runtime", "owner.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var owner coreOwner
	if json.Unmarshal(data, &owner) == nil && owner.Token == token {
		_ = os.Remove(path)
	}
}

// Control routes a command to the private local owner. It never falls back to
// scanning or signalling unrelated OS processes.
func (s *CoreService) Control(ctx context.Context, action, version string) error {
	return s.sendControl(ctx, action, version, nil)
}

func (s *CoreService) sendControl(ctx context.Context, action, version string, activation *ActivateResult) error {
	root, err := config.GetConfigDir()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(root, "runtime", "owner.json"))
	if os.IsNotExist(err) {
		return ErrNoCoreOwner
	}
	if err != nil {
		return err
	}
	var owner coreOwner
	if len(data) > 4096 || json.Unmarshal(data, &owner) != nil || len(owner.Token) != 64 {
		return errors.New("invalid local core owner record")
	}
	u, err := url.Parse(owner.URL)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("invalid local core owner address")
	}
	body, _ := json.Marshal(coreControlRequest{action, version})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, owner.URL+"/control", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+owner.Token)
	req.Header.Set("Content-Type", "application/json")
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 180 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("local control redirects are forbidden") }}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%w: owner is unreachable", ErrNoCoreOwner)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		var result map[string]string
		_ = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&result)
		if result["error"] != "" {
			return errors.New(result["error"])
		}
		return fmt.Errorf("local core owner returned HTTP %d", resp.StatusCode)
	}
	if activation != nil {
		var response coreControlResponse
		if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&response); err != nil {
			return fmt.Errorf("invalid local core profile response: %w", err)
		}
		if response.Result != "ok" {
			return errors.New("invalid local core profile response")
		}
		activation.BackupName = response.BackupName
		if response.MergeWarning != "" {
			activation.MergeErr = errors.New(response.MergeWarning)
		}
	}
	return nil
}
func (s *CoreService) Wait(ctx context.Context) error {
	for {
		s.mu.Lock()
		process := s.process
		done := s.done
		failure := s.failure
		s.mu.Unlock()
		if process == nil || done == nil {
			return failure
		}
		select {
		case <-ctx.Done():
			return s.Stop()
		case <-done:
			// Restart/upgrade holds mu across replacement. Wait for that operation to
			// finish before deciding whether the foreground owner's lifetime has ended.
			s.mu.Lock()
			if s.done != done {
				s.mu.Unlock()
				continue
			}
			err := s.stopLocked()
			s.mu.Unlock()
			if s.daemon {
				return errors.Join(errors.New("managed service child exited unexpectedly"), err)
			}
			return err
		}
	}
}

func (s *CoreService) Events() <-chan string { return s.ownerEvents }

func (s *CoreService) notifyOwner(action string) {
	select {
	case s.ownerEvents <- action:
	default:
		select {
		case <-s.ownerEvents:
		default:
		}
		select {
		case s.ownerEvents <- action:
		default:
		}
	}
}
