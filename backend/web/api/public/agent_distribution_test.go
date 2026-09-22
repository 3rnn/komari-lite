package public

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentInstallerIsServedByPanelWithoutGitHubURLs(t *testing.T) {
	for _, path := range []string{"/agent/install.sh", "/agent/install.ps1"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		ServeAgentInstaller(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("GET %s status=%d", path, recorder.Code)
		}
		body := recorder.Body.String()
		if !strings.Contains(body, "/agent/download/") {
			t.Fatalf("GET %s does not download the Agent from the panel", path)
		}
		if strings.Contains(body, "github.com") || strings.Contains(body, "api.github.com") {
			t.Fatalf("GET %s still exposes a GitHub download URL", path)
		}
	}
}

// 精简版 Agent 由面板自己发布：data/agent-release/manifest.json 给出允许分发的
// 制品清单与 SHA-256，下载时按清单校验，绝不回源到 GitHub。
func TestAgentDownloadServesPanelLocalReleaseAndVerifiesDigest(t *testing.T) {
	dir := t.TempDir()
	payload := []byte("#!/bin/sh\necho slim-agent\n")
	sum := sha256.Sum256(payload)
	digest := hex.EncodeToString(sum[:])
	if err := os.WriteFile(filepath.Join(dir, "komari-agent-linux-amd64"), payload, 0o640); err != nil {
		t.Fatal(err)
	}
	manifest := map[string]any{
		"version":   "1.0",
		"artifacts": map[string]string{"komari-agent-linux-amd64": digest},
	}
	writeAgentTestManifest(t, dir, manifest)

	stubAgentReleaseDir(t, dir)

	recorder := httptest.NewRecorder()
	ServeAgentDownload(recorder, httptest.NewRequest(http.MethodGet, "/agent/download/komari-agent-linux-amd64", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("X-Komari-Agent-Version"); got != "1.0" {
		t.Fatalf("version header = %q", got)
	}
	if recorder.Body.String() != string(payload) {
		t.Fatalf("payload mismatch")
	}
}

func TestAgentDownloadRejectsTamperedArtifact(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "komari-agent-linux-amd64"), []byte("tampered"), 0o640); err != nil {
		t.Fatal(err)
	}
	writeAgentTestManifest(t, dir, map[string]any{
		"version":   "1.0",
		"artifacts": map[string]string{"komari-agent-linux-amd64": strings.Repeat("a", 64)},
	})
	stubAgentReleaseDir(t, dir)

	recorder := httptest.NewRecorder()
	ServeAgentDownload(recorder, httptest.NewRequest(http.MethodGet, "/agent/download/komari-agent-linux-amd64", nil))
	if recorder.Code == http.StatusOK {
		t.Fatalf("tampered artifact was served with status 200")
	}
}

func TestAgentDownloadRejectsUnknownArtifacts(t *testing.T) {
	dir := t.TempDir()
	writeAgentTestManifest(t, dir, map[string]any{
		"version":   "1.0",
		"artifacts": map[string]string{"komari-agent-linux-amd64": strings.Repeat("a", 64)},
	})
	stubAgentReleaseDir(t, dir)

	for _, name := range []string{"not-an-agent", "../manifest.json", "komari-agent-linux-amd64.sh"} {
		recorder := httptest.NewRecorder()
		ServeAgentDownload(recorder, httptest.NewRequest(http.MethodGet, "/agent/download/"+name, nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("artifact %q status=%d, want 404", name, recorder.Code)
		}
	}
}

func TestAgentDownloadFailsLoudlyWithoutManifest(t *testing.T) {
	stubAgentReleaseDir(t, t.TempDir()) // 目录存在但没有 manifest.json
	recorder := httptest.NewRecorder()
	ServeAgentDownload(recorder, httptest.NewRequest(http.MethodGet, "/agent/download/komari-agent-linux-amd64", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want 503 when the panel has no release manifest", recorder.Code)
	}
}

func TestAgentDistributionNeverReferencesGitHub(t *testing.T) {
	source, err := os.ReadFile("agent_distribution.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(source), "github.com/nuomiiiii") {
		t.Fatalf("distribution still fetches from GitHub")
	}
	if !strings.Contains(string(source), "agentReleaseDir") || !strings.Contains(string(source), "manifest.json") {
		t.Fatalf("distribution is not reading a panel-local release directory")
	}
}

func writeAgentTestManifest(t *testing.T, dir string, payload map[string]any) {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0o640); err != nil {
		t.Fatal(err)
	}
}

func stubAgentReleaseDir(t *testing.T, dir string) {
	t.Helper()
	previous := agentReleaseDir
	agentReleaseDir = dir
	t.Cleanup(func() { agentReleaseDir = previous })
}
