package public

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/pkg/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRemoveFaviconIfHashMatches(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "favicon.ico")
	legacyData := []byte("legacy default favicon")
	customData := []byte("custom favicon")
	legacyHash := sha256.Sum256(legacyData)

	if err := os.WriteFile(filePath, legacyData, 0644); err != nil {
		t.Fatal(err)
	}
	removed, err := removeFaviconIfHashMatches(filePath, legacyHash)
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Fatal("legacy default favicon was not removed")
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("legacy favicon still exists: %v", err)
	}

	if err := os.WriteFile(filePath, customData, 0644); err != nil {
		t.Fatal(err)
	}
	removed, err = removeFaviconIfHashMatches(filePath, legacyHash)
	if err != nil {
		t.Fatal(err)
	}
	if removed {
		t.Fatal("custom favicon was removed")
	}
	got, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(customData) {
		t.Fatalf("custom favicon changed: got %q", got)
	}
}

func TestIsSafePathAllowsDotDotFilenameButRejectsTraversal(t *testing.T) {
	base := filepath.Join(t.TempDir(), "themes")
	if !isSafePath(base, "..preview.png") {
		t.Fatal("a filename beginning with two dots stays inside the theme directory")
	}
	if isSafePath(base, filepath.Join("..", "other-theme", "index.html")) {
		t.Fatal("parent-directory traversal must be rejected")
	}
}

func TestStaticThemeFilesRejectGlassRollbackBackup(t *testing.T) {
	t.Chdir(t.TempDir())
	gin.SetMode(gin.TestMode)
	const backupID = "Glass.backup-2026-10-02T00-00-00.000Z-deadbeef"
	for themeID, content := range map[string]string{
		DefaultTheme: "active-theme-asset",
		backupID:     "rollback-theme-asset",
	} {
		assetDir := filepath.Join(DataDir, ThemesDir, themeID, DistDir)
		if err := os.MkdirAll(assetDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(assetDir, "asset.js"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	router := gin.New()
	Static(router.Group("/"), router.NoRoute)
	for _, tc := range []struct {
		id, body string
		status   int
	}{
		{DefaultTheme, "active-theme-asset", http.StatusOK},
		{backupID, "", http.StatusNotFound},
	} {
		t.Run(tc.id, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/themes/"+tc.id+"/dist/asset.js", nil))
			if recorder.Code != tc.status || recorder.Body.String() != tc.body {
				t.Fatalf("theme %q: status=%d body=%q, want status=%d body=%q", tc.id, recorder.Code, recorder.Body.String(), tc.status, tc.body)
			}
		})
	}
	if got, err := os.ReadFile(filepath.Join(DataDir, ThemesDir, backupID, DistDir, "asset.js")); err != nil || string(got) != "rollback-theme-asset" {
		t.Fatalf("rollback asset was not retained on disk: content=%q err=%v", got, err)
	}
}

func TestEmbeddedThemeSourcesContainNoHanCharacters(t *testing.T) {
	for _, root := range []string{"bundledThemes/Glass", "rescueTheme"} {
		err := fs.WalkDir(PublicFS, root, func(name string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() {
				return walkErr
			}
			switch filepath.Ext(name) {
			case ".html", ".js", ".json", ".css":
			default:
				return nil
			}
			content, err := fs.ReadFile(PublicFS, name)
			if err != nil {
				return err
			}
			for _, r := range string(content) {
				if unicode.Is(unicode.Han, r) {
					t.Errorf("%s contains a Han character; keep generated display copy in English", name)
					break
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestEmbeddedGlassUsesEnglishRegionAndVersionedChunk(t *testing.T) {
	const root = "bundledThemes/Glass/dist"
	index, err := fs.ReadFile(PublicFS, root+"/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), `\"lang\":\"en-US\"`) {
		t.Fatal("the embedded page payload must agree with its English HTML language")
	}
	paths, err := fs.Glob(PublicFS, root+"/_next/static/chunks/3859ru-*.js")
	if err != nil {
		t.Fatal(err)
	}
	var referenced []string
	for _, path := range paths {
		if strings.Contains(string(index), "/_next/static/chunks/"+filepath.Base(path)) {
			referenced = append(referenced, path)
		}
	}
	if len(referenced) != 1 {
		t.Fatalf("expected exactly one referenced Glass node-card chunk, got %d", len(referenced))
	}
	path := referenced[0]
	chunk, err := fs.ReadFile(PublicFS, path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(chunk), `function ro(e,t="en")`) {
		t.Fatal("the public region formatter must display English by default")
	}
	sum := sha256.Sum256(chunk)
	name := fmt.Sprintf("3859ru-%x.js", sum[:4])
	if filepath.Base(path) != name || !regexp.MustCompile(`3859ru-[0-9a-f]{8}\.js`).MatchString(name) {
		t.Fatal("the referenced Glass chunk needs a content-derived URL")
	}
}

func TestNormalizeHTMLLanguage(t *testing.T) {
	tests := map[string]struct {
		input string
		want  string
	}{
		"hyphen language": {
			input: "zh-CN",
			want:  "zh-CN",
		},
		"underscore language": {
			input: "zh_CN",
			want:  "zh-CN",
		},
		"reject script injection": {
			input: `zh-CN" autofocus`,
		},
		"reject too short": {
			input: "z",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := normalizeHTMLLanguage(tt.input); got != tt.want {
				t.Fatalf("normalizeHTMLLanguage(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestReplaceHTMLLanguage(t *testing.T) {
	tests := map[string]struct {
		html     string
		language string
		want     string
	}{
		"replace existing lang": {
			html:     `<html lang="en"><head></head></html>`,
			language: "zh-CN",
			want:     `<html lang="zh-CN"><head></head></html>`,
		},
		"replace existing Simplified Chinese lang": {
			html:     `<html lang="zh-CN"><head></head></html>`,
			language: "en-US",
			want:     `<html lang="en-US"><head></head></html>`,
		},
		"insert missing lang": {
			html:     `<html><head></head></html>`,
			language: "ja_JP",
			want:     `<html lang="ja-JP"><head></head></html>`,
		},
		"ignore invalid lang": {
			html:     `<html lang="en"><head></head></html>`,
			language: `zh-CN" autofocus`,
			want:     `<html lang="en"><head></head></html>`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := replaceHTMLLanguage(tt.html, tt.language); got != tt.want {
				t.Fatalf("replaceHTMLLanguage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInjectThemeChangeReload(t *testing.T) {
	withBody := injectThemeChangeReload(`<html><body>theme</body></html>`)
	if !strings.Contains(withBody, themeChangeReloadScript+"</body>") {
		t.Fatalf("theme reload listener was not inserted before body close: %q", withBody)
	}
	if got := strings.Count(injectThemeChangeReload(withBody), themeChangeReloadScript); got != 1 {
		t.Fatalf("theme reload listener count = %d, want 1", got)
	}
	withoutBody := injectThemeChangeReload(`<html>theme</html>`)
	if !strings.HasSuffix(withoutBody, themeChangeReloadScript) {
		t.Fatalf("theme reload listener was not appended: %q", withoutBody)
	}
}

func TestInjectCustomHTML(t *testing.T) {
	got := injectCustomHTML(
		`<HTML><HEAD></HEAD><BODY><main></main></BODY></HTML>`,
		`<style data-custom-head></style>`,
		`<div data-custom-body></div>`,
	)
	if !strings.Contains(got, `<style data-custom-head></style></HEAD>`) {
		t.Fatalf("custom Head content was not inserted before the closing tag: %q", got)
	}
	if !strings.Contains(got, `<div data-custom-body></div></BODY>`) {
		t.Fatalf("custom Body content was not inserted before the closing tag: %q", got)
	}
}

func TestRenderPublicDocumentTitle(t *testing.T) {
	tests := map[string]struct {
		html  string
		title string
		want  string
	}{
		"replace legacy title": {
			html:  `<html><head><title>Komari Monitor</title></head><body></body></html>`,
			title: "Nomi",
			want:  `<title>Nomi</title>`,
		},
		"replace title with attributes and whitespace": {
			html:  "<html><head><TITLE data-theme=\"nezha\">\n Komari Monitor \n</TITLE></head><body></body></html>",
			title: "Nomi",
			want:  `<title>Nomi</title>`,
		},
		"insert missing title": {
			html:  `<html><head><meta charset="utf-8"></head><body></body></html>`,
			title: "Nomi",
			want:  `<meta charset="utf-8"><title>Nomi</title></head>`,
		},
		"escape title markup": {
			html:  `<html><head><title>old</title></head><body></body></html>`,
			title: `Nomi </title><script>alert(1)</script>`,
			want:  `<title>Nomi &lt;/title&gt;&lt;script&gt;alert(1)&lt;/script&gt;</title>`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := renderPublicDocumentTitle(tt.html, tt.title)
			if !strings.Contains(got, tt.want) {
				t.Fatalf("renderPublicDocumentTitle() = %q, want fragment %q", got, tt.want)
			}
			if strings.Count(got, documentTitleSyncMarker) != 1 {
				t.Fatalf("title synchronization marker count = %d, want 1", strings.Count(got, documentTitleSyncMarker))
			}
			if strings.Contains(got, `const expectedTitle="Nomi </title>`) {
				t.Fatalf("title was embedded into script without safe escaping: %q", got)
			}
			if rerendered := renderPublicDocumentTitle(got, tt.title); strings.Count(rerendered, documentTitleSyncMarker) != 1 {
				t.Fatalf("title synchronization was injected more than once: %q", rerendered)
			}
		})
	}
}

func TestRenderApplicationIdentityUsesBackendNameAndFavicon(t *testing.T) {
	htmlStr := `<html><head>
<title>Theme title</title>
<meta name="apple-mobile-web-app-title" content="Theme application" />
<link rel="shortcut icon" href="relative-favicon.ico" />
<link rel="icon" type="image/png" href="/theme-icon.png" />
<link rel="apple-touch-icon" href="/theme-touch-icon.png" />
</head><body></body></html>`

	got := renderApplicationIdentity(htmlStr, `Nomi & Friends`)
	for _, want := range []string{
		`<title>Nomi &amp; Friends</title>`,
		`<meta name="apple-mobile-web-app-title" content="Nomi &amp; Friends" />`,
		`<link rel="icon" href="/favicon.ico" />`,
		`<link rel="apple-touch-icon" href="/favicon.ico" />`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderApplicationIdentity() = %q, want fragment %q", got, want)
		}
	}
	for _, stale := range []string{"Theme application", "/theme-icon.png", "/theme-touch-icon.png"} {
		if strings.Contains(got, stale) {
			t.Fatalf("renderApplicationIdentity() retained stale metadata %q: %q", stale, got)
		}
	}
	if strings.Count(got, `<link rel="icon" href="/favicon.ico" />`) != 1 {
		t.Fatalf("renderApplicationIdentity() did not normalize favicon declarations: %q", got)
	}
	if strings.Contains(got, "relative-favicon.ico") {
		t.Fatalf("renderApplicationIdentity() retained a route-relative favicon: %q", got)
	}
}

func TestRenderSystemApplicationIdentityLeavesRuntimeTitleOwnershipToReact(t *testing.T) {
	got := renderSystemApplicationIdentity(
		`<html><head><title>Komari Lite</title><link rel="shortcut icon" href="favicon.ico" /></head><body></body></html>`,
		"My Komari",
	)
	if !strings.Contains(got, `<title>My Komari</title>`) {
		t.Fatalf("system document did not receive its initial title: %q", got)
	}
	if strings.Contains(got, documentTitleSyncMarker) || strings.Contains(got, "MutationObserver") {
		t.Fatalf("system document retained the public title synchronizer: %q", got)
	}
	if !strings.Contains(got, `<link rel="icon" href="/favicon.ico" />`) || strings.Contains(got, `href="favicon.ico"`) {
		t.Fatalf("system document did not receive a route-safe favicon: %q", got)
	}
}

func TestCustomHTMLIsDisabledOnAllPages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	config.SetDb(db)
	if err := config.SetMany(map[string]any{
		config.SitenameKey:   "My Komari",
		config.CustomHeadKey: `<style data-custom-head>body{--custom-marker:1}</style>`,
		config.CustomBodyKey: `<div data-custom-body>custom body marker</div>`,
	}); err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	Static(router.Group("/"), router.NoRoute)

	tests := []struct {
		path       string
		wantCustom bool
	}{
		{path: "/"},
		{path: "/index.html"},
		{path: "/admin"},
		{path: "/admin/settings"},
		{path: "/terminal"},
		{path: "/terminal/session"},
		{path: "/install"},
		{path: "/manage"},
	}

	for _, tt := range tests {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, tt.path, nil)
		router.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want %d", tt.path, recorder.Code, http.StatusOK)
		}
		body := recorder.Body.String()
		hasCustomHead := strings.Contains(body, `data-custom-head`)
		hasCustomBody := strings.Contains(body, `data-custom-body`)
		if hasCustomHead != tt.wantCustom || hasCustomBody != tt.wantCustom {
			t.Fatalf("GET %s custom HTML = (head: %t, body: %t), want both %t", tt.path, hasCustomHead, hasCustomBody, tt.wantCustom)
		}
		expectedTitle := "My Komari"
		if isAdminApplicationPath(tt.path) {
			expectedTitle = adminApplicationTitle
		}
		for _, want := range []string{
			`<title>` + expectedTitle + `</title>`,
			`<meta name="apple-mobile-web-app-title" content="` + expectedTitle + `" />`,
			`<link rel="icon" href="/favicon.ico" />`,
			`<link rel="apple-touch-icon" href="/favicon.ico" />`,
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("GET %s body does not contain %q", tt.path, want)
			}
		}
		if !isPrivateApplicationPath(tt.path) {
			if !strings.Contains(body, documentTitleSyncMarker) {
				t.Fatalf("GET %s public document has no title synchronizer", tt.path)
			}
		} else if strings.Contains(body, documentTitleSyncMarker) {
			t.Fatalf("GET %s private system document contains the public title synchronizer", tt.path)
		}
		if got := recorder.Header().Get("Cache-Control"); got != "no-store, no-cache, must-revalidate" {
			t.Fatalf("GET %s Cache-Control = %q", tt.path, got)
		}
	}
}

func TestAdminApplicationPath(t *testing.T) {
	tests := map[string]bool{
		"/admin":         true,
		"/admin/servers": true,
		"/administrator": false,
		"/terminal":      false,
		"/install":       false,
		"/manage":        false,
	}
	for requestPath, want := range tests {
		if got := isAdminApplicationPath(requestPath); got != want {
			t.Fatalf("isAdminApplicationPath(%q) = %t, want %t", requestPath, got, want)
		}
	}
}

func TestStaticServesOneDynamicManifestForPublicAndSystemUI(t *testing.T) {
	t.Chdir(t.TempDir())
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	config.SetDb(db)
	if err := config.SetMany(map[string]any{
		config.SitenameKey:    "My Komari",
		config.DescriptionKey: "My monitor",
	}); err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	Static(router.Group("/"), router.NoRoute)

	for _, requestPath := range []string{"/manifest.json", "/system-assets/manifest.json"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, requestPath, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want %d", requestPath, recorder.Code, http.StatusOK)
		}
		if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
			t.Fatalf("GET %s Content-Type = %q", requestPath, got)
		}
		if got := recorder.Header().Get("Cache-Control"); got != "no-store, no-cache, must-revalidate" {
			t.Fatalf("GET %s Cache-Control = %q", requestPath, got)
		}

		var manifest webAppManifest
		if err := json.Unmarshal(recorder.Body.Bytes(), &manifest); err != nil {
			t.Fatalf("decode GET %s: %v", requestPath, err)
		}
		if manifest.ID != "/" || manifest.Name != "My Komari" || manifest.ShortName != "My Komari" {
			t.Fatalf("GET %s identity = (%q, %q, %q)", requestPath, manifest.ID, manifest.Name, manifest.ShortName)
		}
		if manifest.Description != "My monitor" || manifest.StartURL != "/" || manifest.Scope != "/" {
			t.Fatalf("GET %s routing metadata = %#v", requestPath, manifest)
		}
		if len(manifest.Icons) != 1 || manifest.Icons[0] != (webAppManifestIcon{
			Src:     "/favicon.ico",
			Sizes:   "any",
			Type:    "image/x-icon",
			Purpose: "any",
		}) {
			t.Fatalf("GET %s icons = %#v", requestPath, manifest.Icons)
		}
	}
}

func TestPublicFSEmbedsOnlyTheBundledGlassTheme(t *testing.T) {
	themes, err := fs.Glob(PublicFS, "bundledThemes/*")
	if err != nil {
		t.Fatal(err)
	}
	if len(themes) != 1 || themes[0] != "bundledThemes/"+DefaultTheme {
		t.Fatalf("bundled themes = %#v, want only bundledThemes/%s", themes, DefaultTheme)
	}
	if _, err := fs.Stat(PublicFS, bundledThemeRoot+"/komari-theme.json"); err != nil {
		t.Fatalf("bundled theme manifest is missing: %v", err)
	}
	// Next.js exports _next with a leading underscore; go:embed needs all: to include it.
	chunks, err := fs.Glob(PublicFS, bundledThemeRoot+"/dist/_next/static/chunks/*.js")
	if err != nil || len(chunks) == 0 {
		t.Fatalf("bundled theme _next assets are missing from the embed: %v", err)
	}
	if _, err := fs.Stat(PublicFS, bundledThemeRoot+"/dist/"+IndexFile); err != nil {
		t.Fatalf("bundled theme index.html is missing: %v", err)
	}
	// The removed Nezha theme must not remain in the embedded files.
	retired, err := fs.Glob(PublicFS, "bundledThemes/"+RetiredThemeID+"/*")
	if err != nil {
		t.Fatal(err)
	}
	if len(retired) != 0 {
		t.Fatalf("retired theme is still embedded: %#v", retired)
	}
}

func TestEnsureBundledThemesUsesBundledGlassForNewInstall(t *testing.T) {
	t.Chdir(t.TempDir())
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	config.SetDb(db)

	if err := EnsureBundledThemes(); err != nil {
		t.Fatal(err)
	}
	active, err := config.GetAs[string](config.ThemeKey)
	if err != nil {
		t.Fatal(err)
	}
	if active != DefaultTheme {
		t.Fatalf("active theme = %q, want %q", active, DefaultTheme)
	}
	if !IsLocalThemeUsable(DefaultTheme) {
		t.Fatal("bundled Glass theme was not installed")
	}
}

func TestEnsureBundledThemesMigratesLegacyDefaultToBundledGlass(t *testing.T) {
	t.Chdir(t.TempDir())
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	config.SetDb(db)
	if err := config.Set(config.ThemeKey, LegacyDefaultTheme); err != nil {
		t.Fatal(err)
	}

	if err := EnsureBundledThemes(); err != nil {
		t.Fatal(err)
	}
	active, err := config.GetAs[string](config.ThemeKey)
	if err != nil {
		t.Fatal(err)
	}
	if active != DefaultTheme {
		t.Fatalf("active theme = %q, want %q", active, DefaultTheme)
	}
	if !IsLocalThemeUsable(DefaultTheme) {
		t.Fatal("legacy migration did not install the bundled Glass theme")
	}
	if IsLocalThemeUsable("komari-classic") {
		t.Fatal("legacy migration unexpectedly installed the independent Classic theme")
	}
}

func TestEnsureBundledThemesRepairsRestoreWithoutThemeFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	config.SetDb(db)
	if err := config.Set(config.ThemeKey, "missing-after-restore"); err != nil {
		t.Fatal(err)
	}

	if err := EnsureBundledThemes(); err != nil {
		t.Fatal(err)
	}
	active, err := config.GetAs[string](config.ThemeKey)
	if err != nil {
		t.Fatal(err)
	}
	if active != DefaultTheme || !IsLocalThemeUsable(DefaultTheme) {
		t.Fatalf("restored theme state = %q usable=%t", active, IsLocalThemeUsable(DefaultTheme))
	}
}

func TestEnsureBundledThemesRemovesRetiredTheme(t *testing.T) {
	t.Chdir(t.TempDir())
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	config.SetDb(db)

	// Older deployments may still contain an installed copy of Nezha.
	retiredDir := filepath.Join(DataDir, ThemesDir, RetiredThemeID, DistDir)
	if err := os.MkdirAll(retiredDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(DataDir, ThemesDir, RetiredThemeID, "komari-theme.json"), []byte(`{"short":"nezha"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(retiredDir, IndexFile), []byte("retired-theme-index"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Deployments that selected Nezha should fall back to the built-in theme.
	if err := config.Set(config.ThemeKey, RetiredThemeID); err != nil {
		t.Fatal(err)
	}

	if err := EnsureBundledThemes(); err != nil {
		t.Fatal(err)
	}
	if IsLocalThemeUsable(RetiredThemeID) {
		t.Fatal("retired Nezha theme was not removed")
	}
	if _, err := os.Stat(filepath.Join(DataDir, ThemesDir, RetiredThemeID)); !os.IsNotExist(err) {
		t.Fatalf("retired theme directory still present: %v", err)
	}
	active, err := config.GetAs[string](config.ThemeKey)
	if err != nil {
		t.Fatal(err)
	}
	if active != DefaultTheme || !IsLocalThemeUsable(DefaultTheme) {
		t.Fatalf("active theme = %q, bundled usable = %t", active, IsLocalThemeUsable(DefaultTheme))
	}
}

func TestEnsureBundledThemesResetsThirdPartyThemeToFixedGlass(t *testing.T) {
	t.Chdir(t.TempDir())
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	config.SetDb(db)

	customDir := filepath.Join(DataDir, ThemesDir, "third-party", DistDir)
	if err := os.MkdirAll(customDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(DataDir, ThemesDir, "third-party", "komari-theme.json"), []byte(`{"short":"third-party"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(customDir, IndexFile), []byte("third-party-theme"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := config.Set(config.ThemeKey, "third-party"); err != nil {
		t.Fatal(err)
	}

	if err := EnsureBundledThemes(); err != nil {
		t.Fatal(err)
	}
	if !IsLocalThemeUsable(DefaultTheme) {
		t.Fatal("fixed Glass theme was not installed")
	}
	active, err := config.GetAs[string](config.ThemeKey)
	if err != nil || active != DefaultTheme {
		t.Fatalf("active theme = %q, err=%v", active, err)
	}
}

func TestStaticKeepsSystemUIAndPublicThemeResourcesIsolated(t *testing.T) {
	t.Chdir(t.TempDir())
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	config.SetDb(db)
	if err := config.SetMany(map[string]any{
		config.ThemeKey:      "missing-theme",
		config.CustomHeadKey: `<meta data-public-custom-head>`,
		config.CustomBodyKey: `<div data-public-custom-body></div>`,
	}); err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	Static(router.Group("/"), router.NoRoute)

	request := func(requestPath string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, requestPath, nil))
		return recorder
	}

	publicPage := request("/")
	if publicPage.Code != http.StatusOK || strings.Contains(publicPage.Body.String(), "data-public-custom-head") || strings.Contains(publicPage.Body.String(), "data-public-custom-body") {
		t.Fatalf("public rescue page must not execute custom HTML: status=%d body=%q", publicPage.Code, publicPage.Body.String())
	}
	if !strings.Contains(publicPage.Body.String(), "font-logos") {
		t.Fatal("missing public theme did not use the embedded rescue page")
	}

	adminPage := request("/admin/settings/theme")
	if adminPage.Code != http.StatusOK {
		t.Fatalf("system UI status=%d", adminPage.Code)
	}
	if !strings.Contains(adminPage.Body.String(), "/system-assets/") {
		t.Fatal("system UI did not reference its independent asset prefix")
	}
	if strings.Contains(adminPage.Body.String(), "data-public-custom-head") || strings.Contains(adminPage.Body.String(), "font-logos") {
		t.Fatal("public theme content leaked into the system UI")
	}

	entries, err := fs.Glob(PublicFS, "systemUI/dist/assets/entry-*.js")
	if err != nil || len(entries) == 0 {
		t.Fatalf("find embedded system UI entry: %v", err)
	}
	assetPath := "/system-assets/" + strings.TrimPrefix(entries[0], "systemUI/dist/")
	if asset := request(assetPath); asset.Code != http.StatusOK {
		t.Fatalf("GET %s status=%d", assetPath, asset.Code)
	}
	if favicon := request("/favicon.ico"); favicon.Code != http.StatusOK || favicon.Header().Get("Content-Type") != "image/x-icon" {
		t.Fatalf("system favicon fallback status=%d content-type=%q", favicon.Code, favicon.Header().Get("Content-Type"))
	}
	for _, missing := range []string{
		"/system-assets/assets/not-present.js",
		"/themes/Glass/dist/not-present.js",
		"/assets/not-present.js",
	} {
		if response := request(missing); response.Code != http.StatusNotFound {
			t.Fatalf("GET %s status=%d, want 404", missing, response.Code)
		}
	}
}
