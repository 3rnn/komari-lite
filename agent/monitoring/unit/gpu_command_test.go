package monitoring

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestGPUCommandHasDeadline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	path := filepath.Join(t.TempDir(), "gpu")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexec sleep 8\n"), 0700); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { defer close(done); smi := NvidiaSMI{BinPath: path}; _ = smi.Start() }()
	select {
	case <-done:
	case <-time.After(4 * time.Second):
		t.Fatal("GPU command exceeded deadline")
	}
}
