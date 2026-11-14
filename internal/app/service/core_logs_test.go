package service

import "testing"

func TestReadCoreLogsDoesNotWaitForOperationAndKeepsExitOutput(t *testing.T) {
	svc := NewCoreService()
	defer svc.Close()
	svc.mu.Lock()
	svc.logging.Store(true)
	_, _ = svc.logs.Write([]byte("startup\n"))
	output, start, running := svc.ReadLogs(0)
	svc.mu.Unlock()
	if output != "startup\n" || start != 0 || !running {
		t.Fatal("startup output unavailable")
	}
	svc.logging.Store(false)
	_, _ = svc.logs.Write([]byte("final error"))
	output, start, running = svc.ReadLogs(uint64(len(output)))
	if output != "final error" || start != 8 || running {
		t.Fatal("exit output unavailable")
	}
}
