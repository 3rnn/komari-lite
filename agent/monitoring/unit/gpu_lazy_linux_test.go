//go:build linux

package monitoring

import (
	"os"
	"os/exec"
	"testing"
)

func TestGPUVendorDetectionIsLazy(t *testing.T) {
	if os.Getenv("KOMARI_TEST_GPU_LAZY_CHILD") == "1" {
		if vendorType != 0 {
			t.Fatal("GPU vendor was probed at package initialization")
		}
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestGPUVendorDetectionIsLazy$")
	cmd.Env = append(os.Environ(), "KOMARI_TEST_GPU_LAZY_CHILD=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fresh process: %v: %s", err, out)
	}
}
