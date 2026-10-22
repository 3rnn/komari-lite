package accounts

import (
	"errors"
	"image"
	"sync"
	"time"

	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/pquerna/otp/totp"
)

var (
	TwoFactorIssuer = "Komari Monitor"
)

func Generate2Fa() (string, image.Image, error) {
	otp, err := totp.Generate(totp.GenerateOpts{
		Issuer:      TwoFactorIssuer,
		AccountName: "komari",
	})
	if err != nil {
		return "", nil, err
	}
	img, err := otp.Image(250, 250)
	if err != nil {
		return "", nil, err
	}
	return otp.Secret(), img, nil
}

const enrollmentTTL = 10 * time.Minute
const maxPendingEnrollments = 4096

type pendingEnrollment struct {
	uuid, secret, previous string
	expires                time.Time
}

// Restarting the server deliberately cancels unfinished enrollments. Keys are
// non-bearer session hashes, and secrets are never stored in client cookies.
var enrollments = struct {
	sync.Mutex
	entries map[string]pendingEnrollment
}{entries: make(map[string]pendingEnrollment)}

func Begin2FAEnrollment(uuid, session, existingCode string) (image.Image, error) {
	owner, err := GetSession(session)
	if err != nil || owner != uuid || uuid == "" {
		return nil, errors.New("a live account session is required")
	}
	user, err := GetUserByUUID(uuid)
	if err != nil {
		return nil, err
	}
	if user.TwoFactor != "" && !totp.Validate(existingCode, user.TwoFactor) {
		return nil, errors.New("a valid existing 2FA code is required")
	}
	secret, img, err := Generate2Fa()
	if err != nil {
		return nil, err
	}
	enrollments.Lock()
	defer enrollments.Unlock()
	now := time.Now()
	for key, pending := range enrollments.entries {
		if !now.Before(pending.expires) {
			delete(enrollments.entries, key)
		}
	}
	key := SessionID(session)
	if _, exists := enrollments.entries[key]; !exists && len(enrollments.entries) >= maxPendingEnrollments {
		return nil, errors.New("too many pending 2FA enrollments")
	}
	enrollments.entries[key] = pendingEnrollment{uuid: uuid, secret: secret, previous: user.TwoFactor, expires: now.Add(enrollmentTTL)}
	return img, nil
}

func Complete2FAEnrollment(uuid, session, code string) error {
	owner, err := GetSession(session)
	if err != nil || owner != uuid || uuid == "" {
		return errors.New("a live account session is required")
	}
	enrollments.Lock()
	defer enrollments.Unlock()
	key := SessionID(session)
	pending, ok := enrollments.entries[key]
	if !ok || pending.uuid != uuid || !time.Now().Before(pending.expires) {
		delete(enrollments.entries, key)
		return errors.New("2FA enrollment is missing or expired")
	}
	if !totp.Validate(code, pending.secret) {
		return errors.New("invalid 2FA code")
	}
	// A stale enrollment must never overwrite a more recently changed factor.
	result := dbcore.GetDBInstance().Model(&models.User{}).Where("uuid = ? AND COALESCE(two_factor, '') = ?", uuid, pending.previous).Update("two_factor", pending.secret)
	if result.Error != nil {
		return result.Error
	}
	delete(enrollments.entries, key)
	if result.RowsAffected != 1 {
		return errors.New("2FA changed; start enrollment again")
	}
	return nil
}

func Enable2Fa(uuid, secret string) error {
	db := dbcore.GetDBInstance()
	result := db.Model(&models.User{}).Where("uuid = ? AND (two_factor = '' OR two_factor IS NULL)", uuid).Update("two_factor", secret)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("2FA already enabled or account not found")
	}
	return nil
}

func Verify2Fa(uuid, code string) (bool, error) {
	db := dbcore.GetDBInstance()
	var user models.User
	err := db.Where("uuid = ?", uuid).First(&user).Error
	if err != nil {
		return false, err
	}

	if user.TwoFactor == "" {
		return false, nil // User does not enable 2FA
	}

	valid := totp.Validate(code, user.TwoFactor)
	if !valid {
		return false, nil
	}

	return true, nil
}

func Disable2Fa(uuid string) error {
	db := dbcore.GetDBInstance()
	return db.Model(&models.User{}).Where("uuid = ?", uuid).Update("two_factor", "").Error
}
