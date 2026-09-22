package relocate

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"
)

type fakeController struct {
	detectedName string
	detected     bool
	legacy       bool
	running      map[string]bool
	prevented    []string
	disabled     []string
	removed      []string
	installed    *spec
	started      []string
	collect      spec
	startErr     error
}

func (f *fakeController) DetectService(string) (string, bool) {
	return f.detectedName, f.detected
}
func (f *fakeController) LegacyServiceExists(string) bool { return f.legacy }
func (f *fakeController) Collect(string) (spec, error)    { return f.collect, nil }
func (f *fakeController) Install(next spec) error {
	f.installed = &next
	return nil
}
func (f *fakeController) Start(name string) error {
	if f.startErr != nil {
		return f.startErr
	}
	f.started = append(f.started, name)
	if f.running == nil {
		f.running = map[string]bool{}
	}
	f.running[name] = true
	return nil
}
func (f *fakeController) Running(name string) bool { return f.running[name] }
func (f *fakeController) PreventRestart(name string) error {
	f.prevented = append(f.prevented, name)
	return nil
}
func (f *fakeController) DisableNoStop(name string) error {
	f.disabled = append(f.disabled, name)
	return nil
}
func (f *fakeController) StopDisableRemove(name string) error {
	f.removed = append(f.removed, name)
	if f.running != nil {
		f.running[name] = false
	}
	return nil
}

func restoreRelocateHooks() {
	lookupExecutable = os.Executable
	lookupStat = func(path string) error {
		_, err := os.Stat(path)
		return err
	}
	detectPlanFn = detectPlan
}

func sameStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestDoRelocateSkipsContainerCustomServiceAndMissingLegacy(t *testing.T) {
	t.Cleanup(restoreRelocateHooks)
	lookupExecutable = func() (string, error) { return "/opt/komari/agent", nil }

	lookupStat = func(path string) error {
		if path == "/.lite-agent-container" {
			return nil
		}
		return os.ErrNotExist
	}
	ok, err := doRelocate("linux", []string{"/opt/komari/agent"}, nil, &fakeController{legacy: true})
	if err != nil || ok {
		t.Fatalf("container relocate = %v, %v", ok, err)
	}

	lookupStat = func(string) error { return os.ErrNotExist }
	ok, err = doRelocate("linux", []string{"/opt/komari/agent"}, nil, &fakeController{
		detected:     true,
		detectedName: "shop-agent",
		legacy:       true,
	})
	if err != nil || ok {
		t.Fatalf("custom service relocate = %v, %v", ok, err)
	}

	ok, err = doRelocate("linux", []string{"/opt/komari/agent"}, nil, &fakeController{})
	if err != nil || ok {
		t.Fatalf("missing legacy service relocate = %v, %v", ok, err)
	}
}

func TestDoRelocateDoesNotStopSelfWhenNewAlreadyRunning(t *testing.T) {
	t.Cleanup(restoreRelocateHooks)
	oldDir := t.TempDir()
	newDir := t.TempDir()
	src := filepath.Join(oldDir, "agent")
	if err := os.WriteFile(src, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	token := []byte(`{"uuid":"keep-me","token":"secret"}`)
	if err := os.WriteFile(filepath.Join(oldDir, "auto-discovery.json"), token, 0o644); err != nil {
		t.Fatal(err)
	}
	lookupExecutable = func() (string, error) { return src, nil }
	lookupStat = func(string) error { return os.ErrNotExist }
	detectPlanFn = func(string, string, string, string) (plan, bool) {
		return plan{
			From: layout{Dir: oldDir, BinaryName: "agent", Service: "komari-agent"},
			To:   layout{Dir: newDir, BinaryName: "Lite-agent", Service: "lite-agent"},
		}, true
	}
	ctrl := &fakeController{
		legacy:  true,
		running: map[string]bool{"lite-agent": true},
	}
	ok, err := doRelocate("linux", []string{src}, nil, ctrl)
	if err != nil || !ok {
		t.Fatalf("relocate = %v, %v", ok, err)
	}
	if len(ctrl.removed) != 0 {
		t.Fatalf("old process must not stop itself: removed=%v", ctrl.removed)
	}
	if len(ctrl.disabled) != 1 || ctrl.disabled[0] != "komari-agent" {
		t.Fatalf("disabled = %v", ctrl.disabled)
	}
	got, err := os.ReadFile(filepath.Join(newDir, "auto-discovery.json"))
	if err != nil || !bytes.Equal(got, token) {
		t.Fatalf("sidecar copy = %q err=%v", got, err)
	}
}

func TestDoRelocateKeepsOldServiceIfNewDoesNotStart(t *testing.T) {
	t.Cleanup(restoreRelocateHooks)
	oldDir := t.TempDir()
	newDir := t.TempDir()
	src := filepath.Join(oldDir, "agent")
	if err := os.WriteFile(src, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	lookupExecutable = func() (string, error) { return src, nil }
	lookupStat = func(string) error { return os.ErrNotExist }
	detectPlanFn = func(string, string, string, string) (plan, bool) {
		return plan{
			From: layout{Dir: oldDir, BinaryName: "agent", Service: "komari-agent"},
			To:   layout{Dir: newDir, BinaryName: "Lite-agent", Service: "lite-agent"},
		}, true
	}
	ctrl := &fakeController{legacy: true, startErr: os.ErrPermission}
	ok, err := doRelocate("linux", []string{src}, nil, ctrl)
	if err == nil || ok {
		t.Fatalf("expected failed start, got ok=%v err=%v", ok, err)
	}
	if len(ctrl.disabled) != 0 || len(ctrl.removed) != 0 {
		t.Fatalf("old service must stay when new service fails: disabled=%v removed=%v", ctrl.disabled, ctrl.removed)
	}
}

func TestDecodeNssmOutput(t *testing.T) {
	plain := decodeNssmOutput([]byte("-e example.com\r\n"))
	if plain != "-e example.com" {
		t.Fatalf("plain = %q", plain)
	}
	u := utf16.Encode([]rune("-e panel.example.com -t token"))
	buf := make([]byte, len(u)*2)
	for i, v := range u {
		binary.LittleEndian.PutUint16(buf[i*2:], v)
	}
	got := decodeNssmOutput(buf)
	if got != "-e panel.example.com -t token" {
		t.Fatalf("utf16 = %q", got)
	}
}

func TestDoRelocateKeepsAutoDiscoveryMarkerAndIdentityFile(t *testing.T) {
	t.Cleanup(restoreRelocateHooks)

	oldDir := t.TempDir()
	newDir := t.TempDir()
	src := filepath.Join(oldDir, "agent")
	if err := os.WriteFile(src, []byte("old-agent"), 0o755); err != nil {
		t.Fatal(err)
	}
	identity := []byte(`{"uuid":"u","token":"saved-token"}`)
	if err := os.WriteFile(filepath.Join(oldDir, "auto-discovery.json"), identity, 0o644); err != nil {
		t.Fatal(err)
	}

	lookupExecutable = func() (string, error) { return src, nil }
	lookupStat = func(string) error { return os.ErrNotExist }
	detectPlanFn = func(string, string, string, string) (plan, bool) {
		return plan{
			From: layout{Dir: oldDir, BinaryName: "agent", Service: "komari-agent"},
			To:   layout{Dir: newDir, BinaryName: "Lite-agent", Service: "lite-agent"},
		}, true
	}

	ctrl := &fakeController{
		legacy:       true,
		detected:     true,
		detectedName: "komari-agent",
		collect: spec{
			Args:        []string{"-e", "panel.example.com", "--auto-discovery", "legacy-key"},
			Environment: []string{"AGENT_AUTO_DISCOVERY_KEY=legacy-key"},
		},
	}
	ok, err := doRelocate("linux", []string{src, "-e", "panel.example.com", "--auto-discovery", "legacy-key"}, nil, ctrl)
	if err != nil || !ok {
		t.Fatalf("relocate = %v, %v", ok, err)
	}
	if ctrl.installed == nil {
		t.Fatal("expected new service to be installed")
	}
	if !sameStringSlice(ctrl.installed.Args, []string{"-e", "panel.example.com", "--auto-discovery", "legacy-key"}) {
		t.Fatalf("relocated args = %v, want original auto-discovery marker kept", ctrl.installed.Args)
	}
	if !sameStringSlice(ctrl.installed.Environment, []string{"AGENT_AUTO_DISCOVERY_KEY=legacy-key"}) {
		t.Fatalf("relocated env = %v, want original auto-discovery env kept", ctrl.installed.Environment)
	}
	got, err := os.ReadFile(filepath.Join(newDir, "auto-discovery.json"))
	if err != nil || !bytes.Equal(got, identity) {
		t.Fatalf("identity sidecar = %q err=%v", got, err)
	}
}

func TestDoRelocateStopsWhenLegacyIdentityInvalid(t *testing.T) {
	t.Cleanup(restoreRelocateHooks)

	oldDir := t.TempDir()
	newDir := t.TempDir()
	src := filepath.Join(oldDir, "agent")
	if err := os.WriteFile(src, []byte("old-agent"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "auto-discovery.json"), []byte(`{"uuid":"u"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	lookupExecutable = func() (string, error) { return src, nil }
	lookupStat = func(string) error { return os.ErrNotExist }
	detectPlanFn = func(string, string, string, string) (plan, bool) {
		return plan{
			From: layout{Dir: oldDir, BinaryName: "agent", Service: "komari-agent"},
			To:   layout{Dir: newDir, BinaryName: "Lite-agent", Service: "lite-agent"},
		}, true
	}

	ctrl := &fakeController{
		legacy:       true,
		detected:     true,
		detectedName: "komari-agent",
		collect:      spec{Args: []string{"-e", "panel.example.com", "--auto-discovery", "legacy-key"}},
	}
	ok, err := doRelocate("linux", []string{src, "-e", "panel.example.com", "--auto-discovery", "legacy-key"}, nil, ctrl)
	if err == nil || ok {
		t.Fatalf("invalid identity must stop relocation, got ok=%v err=%v", ok, err)
	}
	if ctrl.installed != nil {
		t.Fatal("must not install a new service when saved identity is incomplete")
	}
	if len(ctrl.disabled) != 0 || len(ctrl.removed) != 0 {
		t.Fatalf("must not retire the old service: disabled=%v removed=%v", ctrl.disabled, ctrl.removed)
	}
}
