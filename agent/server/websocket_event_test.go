package server

import (
	"fmt"
	"testing"
)

func resetSeenEventsForTest(t *testing.T) {
	t.Helper()
	v2AckMu.Lock()
	oldEvents := v2SeenEvents
	oldIDs := v2SeenEventIDs
	v2SeenEvents = make(map[string]struct{})
	v2SeenEventIDs = nil
	v2AckMu.Unlock()

	t.Cleanup(func() {
		v2AckMu.Lock()
		v2SeenEvents = oldEvents
		v2SeenEventIDs = oldIDs
		v2AckMu.Unlock()
	})
}

func TestMarkV2EventSeenRejectsDuplicate(t *testing.T) {
	resetSeenEventsForTest(t)

	if !markV2EventSeen("event-1") {
		t.Fatal("first event should be accepted")
	}
	if markV2EventSeen("event-1") {
		t.Fatal("duplicate event should be rejected")
	}
}

func TestMarkV2EventSeenKeepsBoundedHistory(t *testing.T) {
	resetSeenEventsForTest(t)

	for i := 0; i <= v2SeenEventLimit; i++ {
		if !markV2EventSeen(fmt.Sprintf("event-%d", i)) {
			t.Fatalf("event %d was unexpectedly rejected", i)
		}
	}

	v2AckMu.Lock()
	defer v2AckMu.Unlock()
	if len(v2SeenEvents) != v2SeenEventLimit {
		t.Fatalf("seen event map length = %d, want %d", len(v2SeenEvents), v2SeenEventLimit)
	}
	if len(v2SeenEventIDs) != v2SeenEventLimit {
		t.Fatalf("seen event order length = %d, want %d", len(v2SeenEventIDs), v2SeenEventLimit)
	}
	if _, ok := v2SeenEvents["event-0"]; ok {
		t.Fatal("oldest event was not evicted")
	}
}
