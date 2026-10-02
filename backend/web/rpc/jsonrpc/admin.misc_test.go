package jsonrpc

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/rpc"
)

func TestMain(m *testing.M) {
	flags.DatabaseType = flags.DatabaseTypeSQLite
	flags.DatabaseFile = "file:web_rpc_jsonrpc_test?mode=memory&cache=shared"
	os.Exit(m.Run())
}

func TestAdminSessionsHideBearerTokensAndDeleteByID(t *testing.T) {
	db := dbcore.GetDBInstance()
	first := models.Session{UUID: "user-one", Session: "secret-current-bearer", Expires: time.Now().Add(time.Hour)}
	second := models.Session{UUID: "user-two", Session: "secret-other-bearer", Expires: time.Now().Add(time.Hour)}
	for _, s := range []models.Session{first, second} {
		if err := db.Create(&s).Error; err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_ = db.Where("session IN ?", []string{first.Session, second.Session}).Delete(&models.Session{}).Error
	})
	ctx := rpc.NewContextWithMeta(context.Background(), &rpc.ContextMeta{SessionToken: first.Session})
	result, rpcErr := adminGetSessions(ctx, nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{first.Session, second.Session} {
		if strings.Contains(string(encoded), token) {
			t.Fatalf("session list exposed bearer token %q", token)
		}
	}
	var payload struct {
		Current string `json:"current"`
		Data    []struct {
			Session string `json:"session"`
			UUID    string `json:"uuid"`
		} `json:"data"`
	}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Current != accounts.SessionID(first.Session) || len(payload.Data) != 2 {
		t.Fatalf("wrong current ID or session count: %+v", payload)
	}
	ids := map[string]string{}
	for _, s := range payload.Data {
		ids[s.UUID] = s.Session
	}
	if ids[first.UUID] != payload.Current || ids[second.UUID] != accounts.SessionID(second.Session) || ids[first.UUID] == ids[second.UUID] {
		t.Fatalf("session IDs do not identify separate records: %+v", ids)
	}
	// The public ID cannot authenticate, while the real token can.
	if _, err := accounts.GetSession(payload.Current); err == nil {
		t.Fatal("public ID authenticated as a bearer token")
	}
	if _, err := accounts.GetSession(first.Session); err != nil {
		t.Fatal(err)
	}
	// A bearer token supplied to the deletion endpoint must not delete a session.
	if _, rpcErr := adminDeleteSession(ctx, rpc.NewRequest(1, "admin:deleteSession", map[string]any{"session": second.Session})); rpcErr == nil {
		t.Fatal("delete accepted a bearer token in place of an ID")
	}
	if _, err := accounts.GetSession(second.Session); err != nil {
		t.Fatal("bearer-token deletion attempt removed session:", err)
	}
	if _, rpcErr := adminDeleteSession(ctx, rpc.NewRequest(2, "admin:deleteSession", map[string]any{"session": ids[second.UUID]})); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if _, err := accounts.GetSession(second.Session); err == nil {
		t.Fatal("session survived deletion by nonsecret ID")
	}
	if _, err := accounts.GetSession(first.Session); err != nil {
		t.Fatal("deleting another session removed current session:", err)
	}
}

func TestNormalizeAdminDefaultPageSize(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  int
		ok    bool
	}{
		{name: "minimum", value: float64(5), want: 5, ok: true},
		{name: "custom", value: float64(40), want: 40, ok: true},
		{name: "maximum", value: float64(100), want: 100, ok: true},
		{name: "below minimum", value: float64(4), ok: false},
		{name: "above maximum", value: float64(101), ok: false},
		{name: "fraction", value: 10.5, ok: false},
		{name: "wrong type", value: "30", ok: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := normalizeAdminDefaultPageSize(test.value)
			if got != test.want || ok != test.ok {
				t.Fatalf("normalizeAdminDefaultPageSize(%v) = (%d, %v), want (%d, %v)", test.value, got, ok, test.want, test.ok)
			}
		})
	}
}
