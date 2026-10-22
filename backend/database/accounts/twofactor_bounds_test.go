package accounts

import (
	"fmt"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestTwoFactorEnrollmentCapacityAndExpiredCleanup(t *testing.T) {
	uuid, session := enrollmentFixture(t)
	enrollments.Lock()
	previous := enrollments.entries
	enrollments.entries = make(map[string]pendingEnrollment, maxPendingEnrollments)
	for i := 0; i < maxPendingEnrollments; i++ {
		enrollments.entries[fmt.Sprint(i)] = pendingEnrollment{expires: time.Now().Add(enrollmentTTL)}
	}
	enrollments.Unlock()
	t.Cleanup(func() { enrollments.Lock(); enrollments.entries = previous; enrollments.Unlock() })
	if _, err := Begin2FAEnrollment(uuid, session, ""); err == nil {
		t.Fatal("pending enrollment capacity was exceeded")
	}
	enrollments.Lock()
	enrollments.entries["0"] = pendingEnrollment{expires: time.Now().Add(-time.Second)}
	enrollments.Unlock()
	if _, err := Begin2FAEnrollment(uuid, session, ""); err != nil {
		t.Fatal(err)
	}
	// Re-generating for the same session replaces rather than grows the cache.
	if _, err := Begin2FAEnrollment(uuid, session, ""); err != nil {
		t.Fatal(err)
	}
	enrollments.Lock()
	count := len(enrollments.entries)
	enrollments.Unlock()
	if count != maxPendingEnrollments {
		t.Fatalf("cache has %d entries", count)
	}
}

func TestTwoFactorConcurrentEnrollmentsCannotOverwriteWinner(t *testing.T) {
	uuid, first := enrollmentFixture(t)
	second, err := CreateSession(uuid, 3600, "", "", "password")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { DeleteSession(second) })
	codes := make(map[string]string)
	for _, session := range []string{first, second} {
		if _, err := Begin2FAEnrollment(uuid, session, ""); err != nil {
			t.Fatal(err)
		}
		enrollments.Lock()
		pending := enrollments.entries[SessionID(session)]
		enrollments.Unlock()
		codes[session], err = totp.GenerateCode(pending.secret, time.Now())
		if err != nil {
			t.Fatal(err)
		}
	}
	results := make(chan error, 2)
	for _, session := range []string{first, second} {
		go func(session string) { results <- Complete2FAEnrollment(uuid, session, codes[session]) }(session)
	}
	successes := 0
	for i := 0; i < 2; i++ {
		if <-results == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected one atomic enrollment winner, got %d", successes)
	}
	for _, session := range []string{first, second} {
		if err := Complete2FAEnrollment(uuid, session, codes[session]); err == nil {
			t.Fatal("completed or stale enrollment was reusable")
		}
	}
}
