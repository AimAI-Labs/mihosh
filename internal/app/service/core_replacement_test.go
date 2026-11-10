//go:build linux

package service

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/stretchr/testify/require"
)

func markReplacementRunning(s *CoreService, onStop func()) {
	s.process = &exec.Cmd{}
	s.done = make(chan error, 1)
	s.stop = func() {
		if onStop != nil {
			onStop()
		}
		s.done <- nil
	}
}

func replacementBinary(t *testing.T, root, version, script string) string {
	t.Helper()
	path, err := coreinfra.BinaryPath(root, version, "linux")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0700))
	return path
}

func TestCoreReplacementOfflineDoesNotStartProcess(t *testing.T) {
	for _, change := range []string{"version", "tun"} {
		t.Run(change, func(t *testing.T) {
			s, _, _ := prepareManagedProfile(t)
			root, err := config.GetConfigDir()
			require.NoError(t, err)
			if change == "version" {
				replacementBinary(t, root, "v2.0.0", "echo 'Mihomo v2.0.0 linux'\n")
				require.NoError(t, s.ActivateVersion(context.Background(), "v2.0.0"))
			} else {
				require.NoError(t, s.SetTUN(context.Background(), true))
			}
			require.Nil(t, s.process)
			cfg, err := config.Load()
			require.NoError(t, err)
			require.Empty(t, cfg.Core.PendingVersion)
			if change == "version" {
				require.Equal(t, "v2.0.0", cfg.Core.Version)
				require.Equal(t, "v1.2.3", cfg.Core.PreviousVersion)
			} else {
				require.True(t, cfg.Core.TUN)
				_, err = os.Stat(filepath.Join(root, "runtime", "network-change.json"))
				require.True(t, os.IsNotExist(err))
			}
		})
	}
}

func TestCoreReplacementCandidateFailureRestoresPreviousSelection(t *testing.T) {
	for _, change := range []string{"version", "tun"} {
		t.Run(change, func(t *testing.T) {
			s, _, old := prepareManagedProfile(t)
			root, err := config.GetConfigDir()
			require.NoError(t, err)
			markReplacementRunning(s, nil)
			var attempts string
			if change == "version" {
				oldBinary := replacementBinary(t, root, "v1.2.3", "echo recovery > \"$0.restarted\"\nexit 1\n")
				replacementBinary(t, root, "v2.0.0", "if [ \"$1\" = '-v' ]; then echo 'Mihomo v2.0.0 linux'; exit 0; fi\nif [ -f \"$0.validated\" ]; then exit 1; fi\ntouch \"$0.validated\"\n")
				err = s.ActivateVersion(context.Background(), "v2.0.0")
				_, statErr := os.Stat(oldBinary + ".restarted")
				require.NoError(t, statErr, "previous selection was not restarted")
			} else {
				binary := replacementBinary(t, root, "v1.2.3", "echo attempt >> \"$0.attempts\"\nif [ -f \"$0.validated\" ]; then exit 1; fi\ntouch \"$0.validated\"\n")
				err = s.SetTUN(context.Background(), true)
				attempts = binary + ".attempts"
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), "managed core configuration validation failed")
			if attempts != "" {
				data, err := os.ReadFile(attempts)
				require.NoError(t, err)
				require.Equal(t, "attempt\nattempt\nattempt\n", string(data), "validate, candidate start, previous start")
			}
			cfg, err := config.Load()
			require.NoError(t, err)
			require.Equal(t, "v1.2.3", cfg.Core.Version)
			require.Empty(t, cfg.Core.PreviousVersion)
			require.Empty(t, cfg.Core.PendingVersion)
			require.False(t, cfg.Core.TUN)
			data, err := os.ReadFile(filepath.Join(root, "runtime", "config.yaml"))
			require.NoError(t, err)
			require.Equal(t, old, data)
		})
	}
}

func TestCoreReplacementCancellationRestoresPreviousSelection(t *testing.T) {
	for _, change := range []string{"version", "tun"} {
		t.Run(change, func(t *testing.T) {
			s, _, old := prepareManagedProfile(t)
			root, err := config.GetConfigDir()
			require.NoError(t, err)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			markReplacementRunning(s, cancel)
			if change == "version" {
				replacementBinary(t, root, "v2.0.0", "echo 'Mihomo v2.0.0 linux'\n")
				err = s.ActivateVersion(ctx, "v2.0.0")
			} else {
				err = s.SetTUN(ctx, true)
			}
			require.ErrorIs(t, err, context.Canceled)
			cfg, err := config.Load()
			require.NoError(t, err)
			require.Equal(t, "v1.2.3", cfg.Core.Version)
			require.Empty(t, cfg.Core.PendingVersion)
			require.False(t, cfg.Core.TUN)
			data, err := os.ReadFile(filepath.Join(root, "runtime", "config.yaml"))
			require.NoError(t, err)
			require.Equal(t, old, data)
		})
	}
}

func TestTUNReplacementJournalFailureIncludesRestartFailure(t *testing.T) {
	s, _, _ := prepareManagedProfile(t)
	root, err := config.GetConfigDir()
	require.NoError(t, err)
	journal := filepath.Join(root, "runtime", "network-change.json")
	markReplacementRunning(s, func() { require.NoError(t, os.Mkdir(journal, 0700)) })
	replacementBinary(t, root, "v1.2.3", "if [ -f \"$0.validated\" ]; then exit 1; fi\ntouch \"$0.validated\"\n")
	err = s.SetTUN(context.Background(), true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "rename")
	require.Contains(t, err.Error(), "managed core configuration validation failed")
}

func TestCoreReplacementCancellationDuringCandidateValidation(t *testing.T) {
	for _, change := range []string{"version", "tun"} {
		t.Run(change, func(t *testing.T) {
			s, _, old := prepareManagedProfile(t)
			root, err := config.GetConfigDir()
			require.NoError(t, err)
			version := "v1.2.3"
			if change == "version" {
				version = "v2.0.0"
				replacementBinary(t, root, "v1.2.3", "exit 1\n")
			}
			binary := replacementBinary(t, root, version, "if [ \"$1\" = '-v' ]; then echo 'Mihomo v2.0.0 linux'; exit 0; fi\nif [ -f \"$0.candidate\" ]; then exit 1; fi\nif [ -f \"$0.validated\" ]; then touch \"$0.candidate\"; exec sleep 30; fi\ntouch \"$0.validated\"\n")
			markReplacementRunning(s, nil)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			finished := make(chan error, 1)
			go func() {
				if change == "version" {
					finished <- s.ActivateVersion(ctx, version)
				} else {
					finished <- s.SetTUN(ctx, true)
				}
			}()
			require.Eventually(t, func() bool {
				_, err := os.Stat(binary + ".candidate")
				return err == nil
			}, 2*time.Second, 10*time.Millisecond)
			cancel()
			require.ErrorIs(t, <-finished, context.Canceled)
			cfg, err := config.Load()
			require.NoError(t, err)
			require.Equal(t, "v1.2.3", cfg.Core.Version)
			require.Empty(t, cfg.Core.PendingVersion)
			require.False(t, cfg.Core.TUN)
			data, err := os.ReadFile(filepath.Join(root, "runtime", "config.yaml"))
			require.NoError(t, err)
			require.Equal(t, old, data)
		})
	}
}

func TestTUNReplacementRestoreFailureKeepsJournalAndDoesNotRestart(t *testing.T) {
	s, _, _ := prepareManagedProfile(t)
	root, err := config.GetConfigDir()
	require.NoError(t, err)
	markReplacementRunning(s, nil)
	binary := replacementBinary(t, root, "v1.2.3", "echo attempt >> \"$0.attempts\"\nif [ -f \"$3/network-change.json\" ]; then echo invalid > \"$3/network-change.json\"; exit 1; fi\n")
	err = s.SetTUN(context.Background(), true)
	require.ErrorContains(t, err, "invalid network change journal")
	_, err = os.Stat(filepath.Join(root, "runtime", "network-change.json"))
	require.NoError(t, err)
	data, err := os.ReadFile(binary + ".attempts")
	require.NoError(t, err)
	require.Equal(t, "attempt\nattempt\n", string(data))
	require.Nil(t, s.process)
}
