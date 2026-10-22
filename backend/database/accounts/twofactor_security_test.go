package accounts

import (
	"testing"
	"time"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/pquerna/otp/totp"
)

func enrollmentFixture(t *testing.T) (string, string) {
	t.Helper()
	flags.DatabaseFile = "file:account_enrollment_security?mode=memory&cache=shared"
	user, err := CreateAccount(t.Name(), "test-only-password")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { DeleteAccountByUsername(user.Username) })
	token, err := CreateSession(user.UUID, 3600, "", "", "password")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { DeleteSession(token) })
	return user.UUID, token
}

func TestTwoFactorEnrollmentAcceptsLegacyNullFactor(t *testing.T) {
	uuid, session := enrollmentFixture(t)
	if err := dbcore.GetDBInstance().Model(&models.User{}).Where("uuid = ?", uuid).Update("two_factor", nil).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := Begin2FAEnrollment(uuid, session, ""); err != nil {
		t.Fatal(err)
	}
	enrollments.Lock()
	pending := enrollments.entries[SessionID(session)]
	enrollments.Unlock()
	code, err := totp.GenerateCode(pending.secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := Complete2FAEnrollment(uuid, session, code); err != nil {
		t.Fatal(err)
	}
}

func TestEnable2FaCannotReplaceExistingFactor(t *testing.T) {
	uuid, _ := enrollmentFixture(t)
	first, _, err := Generate2Fa()
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := Generate2Fa()
	if err != nil {
		t.Fatal(err)
	}
	if err := Enable2Fa(uuid, first); err != nil {
		t.Fatal(err)
	}
	if err := Enable2Fa(uuid, second); err == nil {
		t.Fatal("low-level enable overwrote existing factor without proof")
	}
	user, err := GetUserByUUID(uuid)
	if err != nil {
		t.Fatal(err)
	}
	if user.TwoFactor != first {
		t.Fatal("existing factor changed")
	}
}

func TestTwoFactorEnrollmentSessionExpiryAndReplacement(t *testing.T) {
	uuid, session := enrollmentFixture(t)
	if _, err := Begin2FAEnrollment(uuid, session, ""); err != nil {
		t.Fatal(err)
	}
	enrollments.Lock()
	pending := enrollments.entries[SessionID(session)]
	enrollments.Unlock()
	code, err := totp.GenerateCode(pending.secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	other, err := CreateSession(uuid, 3600, "", "", "password")
	if err != nil {
		t.Fatal(err)
	}
	defer DeleteSession(other)
	if err := Complete2FAEnrollment(uuid, other, code); err == nil {
		t.Fatal("another session consumed pending enrollment")
	}
	if err := Complete2FAEnrollment(uuid, session, "not-a-code"); err == nil {
		t.Fatal("invalid new-factor proof accepted")
	}
	if err := Complete2FAEnrollment(uuid, session, code); err != nil {
		t.Fatal(err)
	}
	if err := Complete2FAEnrollment(uuid, session, code); err == nil {
		t.Fatal("consumed enrollment replayed")
	}
	if _, err := Begin2FAEnrollment(uuid, session, ""); err == nil {
		t.Fatal("replacement did not require existing factor")
	}
	if _, err := Begin2FAEnrollment(uuid, session, code); err != nil {
		t.Fatal(err)
	}
	enrollments.Lock()
	replacement := enrollments.entries[SessionID(session)]
	enrollments.Unlock()
	replacementCode, err := totp.GenerateCode(replacement.secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := Complete2FAEnrollment(uuid, session, replacementCode); err != nil {
		t.Fatal(err)
	}
	if _, err := Begin2FAEnrollment(uuid, session, replacementCode); err != nil {
		t.Fatal(err)
	}
	enrollments.Lock()
	expired := enrollments.entries[SessionID(session)]
	expired.expires = time.Now().Add(-time.Second)
	enrollments.entries[SessionID(session)] = expired
	enrollments.Unlock()
	expiredCode, err := totp.GenerateCode(expired.secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := Complete2FAEnrollment(uuid, session, expiredCode); err == nil {
		t.Fatal("expired pending enrollment accepted")
	}
}

func TestTwoFactorEnrollmentRejectsRevocationAndStaleFactor(t *testing.T) {
	uuid, session := enrollmentFixture(t)
	if _, err := Begin2FAEnrollment(uuid, session, ""); err != nil {
		t.Fatal(err)
	}
	enrollments.Lock()
	pending := enrollments.entries[SessionID(session)]
	enrollments.Unlock()
	code, err := totp.GenerateCode(pending.secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	factor, _, err := Generate2Fa()
	if err != nil {
		t.Fatal(err)
	}
	if err := Enable2Fa(uuid, factor); err != nil {
		t.Fatal(err)
	}
	if err := Complete2FAEnrollment(uuid, session, code); err == nil {
		t.Fatal("stale enrollment overwrote changed factor")
	}
	existingCode, err := totp.GenerateCode(factor, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Begin2FAEnrollment(uuid, session, existingCode); err != nil {
		t.Fatal(err)
	}
	if err := DeleteSession(session); err != nil {
		t.Fatal(err)
	}
	if err := Complete2FAEnrollment(uuid, session, code); err == nil {
		t.Fatal("revoked session completed enrollment")
	}
}
