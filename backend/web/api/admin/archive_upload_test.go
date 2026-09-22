package admin

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/pkg/rpc"
	"github.com/komari-monitor/komari/utils/httpsserver"
	"github.com/komari-monitor/komari/web/api"
	"github.com/komari-monitor/komari/web/backup"
	"github.com/komari-monitor/komari/web/upload"
)

func themeArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func archiveUploadStore(t *testing.T) *upload.Store {
	t.Helper()
	return &upload.Store{
		Root:                filepath.Join(t.TempDir(), "uploading"),
		MaxSize:             backup.MaxArchiveSize,
		MaxReservedSize:     backup.MaxArchiveSize,
		MaxSessionsPerOwner: 2,
		SessionTTL:          time.Hour,
		Now:                 time.Now,
	}
}

func archiveUploadRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		api.SetPrincipal(ctx, rpc.NewUserPrincipal("test-admin"))
		ctx.Next()
	})
	handler := newArchiveUploadHandler(archiveUploadStore(t))
	group := router.Group("/api/admin/upload")
	group.POST("/init", handler.Init)
	group.POST("/chunk", handler.Chunk)
	group.POST("/merge", handler.Merge)
	group.POST("/cancel", handler.Cancel)
	return router
}

func postJSON(t *testing.T, router http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(data))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func chunkUploadRequest(t *testing.T, path, uploadID string, index int, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	if err := form.WriteField("upload_id", uploadID); err != nil {
		t.Fatal(err)
	}
	if err := form.WriteField("chunk_index", fmt.Sprint(index)); err != nil {
		t.Fatal(err)
	}
	chunk, err := form.CreateFormFile("chunk_data", "chunk.part")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := chunk.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, &body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	return request
}

func uploadArchiveThroughHandler(t *testing.T, router http.Handler, purpose upload.Purpose, filename string, archive []byte) *httptest.ResponseRecorder {
	t.Helper()
	initResponse := postJSON(t, router, "/api/admin/upload/init", map[string]any{
		"purpose": purpose, "filename": filename, "size": len(archive),
	})
	if initResponse.Code != http.StatusOK {
		t.Fatalf("init upload status = %d: %s", initResponse.Code, initResponse.Body.String())
	}
	var initialized struct {
		Data struct {
			UploadID string `json:"upload_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(initResponse.Body.Bytes(), &initialized); err != nil {
		t.Fatal(err)
	}
	chunkRequest := chunkUploadRequest(t, "/api/admin/upload/chunk", initialized.Data.UploadID, 0, archive)
	chunkResponse := httptest.NewRecorder()
	router.ServeHTTP(chunkResponse, chunkRequest)
	if chunkResponse.Code != http.StatusOK {
		t.Fatalf("chunk upload status = %d: %s", chunkResponse.Code, chunkResponse.Body.String())
	}
	return postJSON(t, router, "/api/admin/upload/merge", map[string]string{"upload_id": initialized.Data.UploadID})
}

func runRestartImmediately(t *testing.T) {
	t.Helper()
	oldSchedule, oldExit := scheduleAdminRestart, exitAdminProcess
	scheduleAdminRestart = func(_ time.Duration, task func()) { task() }
	exitAdminProcess = func(int) {}
	t.Cleanup(func() {
		scheduleAdminRestart, exitAdminProcess = oldSchedule, oldExit
	})
}

func TestChunkedBackupUploadStagesOnlyValidatedArchive(t *testing.T) {
	t.Chdir(t.TempDir())
	runRestartImmediately(t)
	router := archiveUploadRouter(t)
	archive := themeArchive(t, map[string]string{
		"komari.db":            "sqlite-data",
		"metrics.db":           "metrics-data",
		"komari-backup-markup": "full",
	})
	response := uploadArchiveThroughHandler(t, router, upload.PurposeBackup, "backup.zip", archive)
	if response.Code != http.StatusOK {
		t.Fatalf("merge backup status = %d: %s", response.Code, response.Body.String())
	}
	staged := filepath.Join("data", "backup.zip")
	if err := backup.ValidateArchive(staged); err != nil {
		t.Fatalf("staged backup is invalid: %v", err)
	}
}

func TestUploadInitRejectsRemovedThemePurpose(t *testing.T) {
	t.Chdir(t.TempDir())
	response := postJSON(t, archiveUploadRouter(t), "/api/admin/upload/init", map[string]any{
		"purpose": upload.PurposeTheme,
		"size":    3,
	})
	if response.Code == http.StatusOK {
		t.Fatalf("removed theme upload purpose was accepted: %s", response.Body.String())
	}
}

func TestChunkedBackupUploadFailurePreservesExistingStagedBackup(t *testing.T) {
	for name, archive := range map[string][]byte{
		"not zip":        []byte("not a zip"),
		"missing marker": themeArchive(t, map[string]string{"komari.db": "sqlite-data"}),
		"missing database": themeArchive(t, map[string]string{
			"komari-backup-markup": "config",
		}),
	} {
		t.Run(name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			runRestartImmediately(t)
			if err := os.MkdirAll("data", 0o755); err != nil {
				t.Fatal(err)
			}
			const oldBackup = "existing staged backup"
			if err := os.WriteFile(filepath.Join("data", "backup.zip"), []byte(oldBackup), 0o600); err != nil {
				t.Fatal(err)
			}
			response := uploadArchiveThroughHandler(t, archiveUploadRouter(t), upload.PurposeBackup, "backup.zip", archive)
			if response.Code == http.StatusOK {
				t.Fatalf("invalid backup was accepted: %s", response.Body.String())
			}
			content, err := os.ReadFile(filepath.Join("data", "backup.zip"))
			if err != nil || string(content) != oldBackup {
				t.Fatalf("existing staged backup changed: content=%q err=%v", content, err)
			}
		})
	}
}

func TestChunkedThemeUploadIsRejected(t *testing.T) {
	t.Chdir(t.TempDir())
	router := archiveUploadRouter(t)
	response := postJSON(t, router, "/api/admin/upload/init", map[string]any{
		"purpose":  upload.PurposeTheme,
		"filename": "uploaded.zip",
		"size":     1,
	})
	if response.Code == http.StatusOK {
		t.Fatalf("removed theme upload was accepted: %s", response.Body.String())
	}
}

func writeArchiveUploadCertificate(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "monitor.example"},
		DNSNames:     []string{"monitor.example"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath
}

func TestChunkedThemeUploadThroughBuiltInHTTPSRedirect(t *testing.T) {
	t.Chdir(t.TempDir())
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		cookie, err := ctx.Cookie("session_token")
		if err != nil || cookie != "built-in-https-session" {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		api.SetPrincipal(ctx, rpc.NewUserPrincipal("https-admin"))
		ctx.Next()
	})
	handler := newArchiveUploadHandler(archiveUploadStore(t))
	group := router.Group("/api/admin/upload")
	group.POST("/init", handler.Init)
	group.POST("/chunk", handler.Chunk)
	group.POST("/merge", handler.Merge)

	certPath, keyPath := writeArchiveUploadCertificate(t)
	manager := httpsserver.NewManager()
	settings := httpsserver.Settings{
		Enabled: true, Listen: "127.0.0.1:0", RedirectHTTP: true,
		CertificatePath: certPath, PrivateKeyPath: keyPath,
	}
	if err := manager.Start(router, settings); err != nil {
		t.Fatalf("start built-in HTTPS: %v", err)
	}
	t.Cleanup(func() { _ = manager.Shutdown(context.Background()) })
	plainServer := httptest.NewServer(manager.HTTPRedirectHandler(router))
	defer plainServer.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	plainURL, err := url.Parse(plainServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	jar.SetCookies(plainURL, []*http.Cookie{{Name: "session_token", Value: "built-in-https-session", Path: "/"}})
	client := &http.Client{
		Jar: jar,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Test-only certificate for monitor.example is intentionally reached through 127.0.0.1.
		}},
	}
	do := func(path, contentType string, body []byte) *http.Response {
		t.Helper()
		request, err := http.NewRequest(http.MethodPost, plainServer.URL+path, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", contentType)
		response, err := client.Do(request)
		if err != nil {
			t.Fatalf("request through built-in HTTPS: %v", err)
		}
		if response.Request.URL.Scheme != "https" {
			response.Body.Close()
			t.Fatalf("request did not finish on built-in HTTPS: %s", response.Request.URL)
		}
		return response
	}

	initBody, _ := json.Marshal(map[string]any{"purpose": "theme", "filename": "https-theme.zip", "size": 1})
	response := do("/api/admin/upload/init", "application/json", initBody)
	defer response.Body.Close()
	if response.StatusCode == http.StatusOK {
		t.Fatalf("removed theme upload was accepted through built-in HTTPS")
	}
}
