//go:build !windows

package tui

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/app/service"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/logs"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/messages"
)

func TestFailedManagedCoreStartupIsVisibleInOfflineLogs(t *testing.T) {
	for _, fixture := range []struct{ name, script, reason string }{
		{"process", "#!/bin/sh\nif [ \"$1\" = -t ]; then exit 0; fi\nprintf 'level=fatal msg=\"startup fixture failed\"'\nexit 1\n", "startup fixture failed"},
		{"validation", "#!/bin/sh\nprintf 'yaml: line 2: fixture validation failed\\n' >&2\nexit 1\n", "yaml: line 2: fixture validation failed"},
	} {
		t.Run(fixture.name, func(t *testing.T) { verifyFailedManagedCoreLogs(t, fixture.script, fixture.reason) })
	}
}

func verifyFailedManagedCoreLogs(t *testing.T, script, reason string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("MIHOSH_CONFIG_DIR", root)
	cfg := config.DefaultConfig
	cfg.Core.Version = "v1.2.3"
	if err := config.Save(&cfg); err != nil {
		t.Fatal(err)
	}
	svc := service.NewCoreService()
	defer svc.Close()
	if err := svc.Configure(); err != nil {
		t.Fatal(err)
	}
	port := func() int {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()
		return listener.Addr().(*net.TCPAddr).Port
	}
	if err := svc.SetPorts(port(), port()); err != nil {
		t.Fatal(err)
	}
	binary, err := coreinfra.BinaryPath(root, cfg.Core.Version, runtime.GOOS)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(binary), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	if err := svc.Start(context.Background()); err == nil {
		t.Fatal("fixture core should fail startup")
	}
	stored, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	m := Model{config: stored, coreSvc: svc, logsState: logs.NewState(nil), currentPage: layout.PageLogs, connectionChecked: true}
	if cmd := m.onPageChange(); cmd == nil {
		t.Fatal("offline logs page did not start its refresh timer")
	}
	if entries := m.logsState.Logs(); len(entries) != 1 || entries[0].Type != "error" || !strings.Contains(entries[0].Payload, reason) {
		t.Fatal("startup output did not reach the logs page", entries)
	}
	m.syncProcessLogs()
	if len(m.logsState.Logs()) != 1 {
		t.Fatal("process log replayed on refresh")
	}
	if view := logs.RenderLogsPage(m.logsState.ToPageState(120, 24)); !strings.Contains(view, reason) && !strings.Contains(view, "managed core configuration validation failed") {
		t.Fatal("startup failure was not rendered")
	}
	updated, _ := m.Update(messages.LogsWSMsg{LogType: "error", Payload: reason})
	m = updated.(Model)
	if len(m.logsState.Logs()) != 1 {
		t.Fatal("owned process output was duplicated by WebSocket")
	}
	m.config.Core.Runtime = "service"
	updated, _ = m.Update(messages.LogsWSMsg{LogType: "info", Payload: "background owner log"})
	m = updated.(Model)
	if len(m.logsState.Logs()) != 2 {
		t.Fatal("previous session suppressed background owner logs")
	}
	m.coreSvc = nil
	updated, _ = m.Update(messages.LogsWSMsg{LogType: "debug", Payload: "external live log"})
	if len(updated.(Model).logsState.Logs()) != 3 {
		t.Fatal("WebSocket logs for other instances were suppressed")
	}
}
