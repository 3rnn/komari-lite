package public

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	agentManifestName  = "manifest.json"
	maxAgentBinarySize = 64 << 20
)

//go:embed agent_installers/*
var agentInstallerFS embed.FS

var (
	// agentReleaseDir 是面板自己发布的 Agent 制品目录（随 data/ 一起备份）。
	// 目录里放 14 个平台的二进制 + manifest.json，目标节点只访问本面板。
	agentReleaseDir = "data/agent-release"

	agentDigestMu    sync.Mutex
	agentDigestCache = map[string]string{}
)

// agentReleaseManifest 描述面板当前发布的精简版 Agent。
// version 会通过 X-Komari-Agent-Version 暴露；artifacts 是「文件名 → SHA-256」白名单，
// 只有清单里的文件才会被分发，且每次下载都按清单校验摘要。
type agentReleaseManifest struct {
	Version   string            `json:"version"`
	Artifacts map[string]string `json:"artifacts"`
}

// ServeAgentInstaller serves the panel-owned installer scripts. Generated
// one-click commands use these routes instead of raw.githubusercontent.com.
func ServeAgentInstaller(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(r.URL.Path)
	if name != "install.sh" && name != "install.ps1" {
		http.NotFound(w, r)
		return
	}
	body, err := agentInstallerFS.ReadFile("agent_installers/" + name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

// ServeAgentDownload 从面板本地发布目录返回一个精简版 Agent 制品。
// 制品清单或摘要对不上时宁可失败，也不分发未经校验的二进制。
func ServeAgentDownload(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(r.URL.Path)
	manifest, err := loadAgentReleaseManifest()
	if err != nil {
		log.Printf("agent distribution unavailable: %v", err)
		http.Error(w, "Agent release is not published on this panel", http.StatusServiceUnavailable)
		return
	}
	expected, allowed := manifest.Artifacts[name]
	if !allowed || strings.ContainsAny(name, `/\`) {
		http.NotFound(w, r)
		return
	}
	artifactPath := filepath.Join(agentReleaseDir, name)
	if !agentArtifactIsTrusted(artifactPath, expected) {
		log.Printf("agent distribution refused %s: digest mismatch or missing file", name)
		http.Error(w, "Agent artifact failed verification", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, name))
	w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	w.Header().Set("X-Komari-Agent-Version", manifest.Version)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, artifactPath)
}

func loadAgentReleaseManifest() (agentReleaseManifest, error) {
	var manifest agentReleaseManifest
	raw, err := os.ReadFile(filepath.Join(agentReleaseDir, agentManifestName))
	if err != nil {
		return manifest, err
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return manifest, fmt.Errorf("parse %s: %w", agentManifestName, err)
	}
	if strings.TrimSpace(manifest.Version) == "" || len(manifest.Artifacts) == 0 {
		return manifest, fmt.Errorf("%s is incomplete", agentManifestName)
	}
	for name, digest := range manifest.Artifacts {
		if strings.ContainsAny(name, `/\`) {
			return manifest, fmt.Errorf("%s lists an illegal artifact name %q", agentManifestName, name)
		}
		if !isSHA256Hex(digest) {
			return manifest, fmt.Errorf("%s lists an invalid digest for %s", agentManifestName, name)
		}
	}
	return manifest, nil
}

// agentArtifactIsTrusted 校验文件存在、大小合理且摘要与清单一致；摘要按文件状态缓存，
// 避免每次下载都重新哈希十几兆的二进制。
func agentArtifactIsTrusted(path, expected string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxAgentBinarySize {
		return false
	}
	cacheKey := fmt.Sprintf("%s|%d|%d", path, info.Size(), info.ModTime().UnixNano())
	agentDigestMu.Lock()
	cached, ok := agentDigestCache[cacheKey]
	agentDigestMu.Unlock()
	if ok {
		return strings.EqualFold(cached, expected)
	}
	matches, err := agentFileMatchesSHA256(path, expected)
	if err != nil {
		return false
	}
	digest, err := agentFileSHA256(path)
	if err != nil {
		return false
	}
	agentDigestMu.Lock()
	agentDigestCache = map[string]string{cacheKey: digest}
	agentDigestMu.Unlock()
	return matches
}

func agentFileMatchesSHA256(path, expected string) (bool, error) {
	digest, err := agentFileSHA256(path)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(digest, expected), nil
}

func agentFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func isSHA256Hex(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
