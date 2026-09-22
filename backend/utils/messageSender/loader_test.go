package messageSender

import (
	"testing"
	"time"

	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/utils/messageSender/factory"
)

func TestParseTemplateFormatsEventTimeAsUTCNanoseconds(t *testing.T) {
	local := time.FixedZone("UTC+8", 8*60*60)
	eventTime := time.Date(2026, 7, 17, 9, 30, 0, 123456789, local)
	got := parseTemplate("{{time}}", models.EventMessage{Time: eventTime})
	want := "2026-07-17T01:30:00.123456789Z"
	if got != want {
		t.Fatalf("formatted event time = %q, want %q", got, want)
	}
}

func Test(t *testing.T) {
	senders := factory.GetAllMessageSenders()
	if len(senders) == 0 {
		t.Error("No message senders found")
		return
	}
	cfg := factory.GetSenderConfigs()
	if len(cfg) == 0 {
		t.Error("No sender configs found")
		return
	}
	// Only Telegram (and the no-op "empty" sender) ship in this monitoring build.
	if err := LoadProvider("telegram", `{"bot_token":"","chat_id":""}`); err != nil {
		t.Fatalf("loading the compiled-in telegram provider failed: %v", err)
	}
	cp := CurrentProvider
	if cp() == nil {
		t.Error("Current provider is nil")
		return
	}
	// A provider name that was stripped from the build must return an error
	// instead of leaving the notification channel uninitialized or panicking.
	if err := LoadProvider("email", `{}`); err == nil {
		t.Error("removed provider email should not load in the monitoring build")
	}
	if CurrentProvider() == nil {
		t.Error("failed provider load must keep the previous provider alive")
	}
}
