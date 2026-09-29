package accounts

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"gorm.io/gorm"
)

const constantSalt = "06Wm4Jv1Hkxx"

var allowedPreferenceLanguages = map[string]struct{}{
	"en-US": {},
	"zh-CN": {},
	"zh-TW": {},
	"ja-JP": {},
	"id-ID": {},
}

var allowedPreferenceColors = map[string]struct{}{
	"gray": {}, "gold": {}, "bronze": {}, "brown": {}, "yellow": {}, "amber": {},
	"orange": {}, "tomato": {}, "red": {}, "ruby": {}, "crimson": {}, "pink": {},
	"plum": {}, "purple": {}, "violet": {}, "iris": {}, "indigo": {}, "blue": {},
	"cyan": {}, "teal": {}, "jade": {}, "green": {}, "grass": {}, "lime": {},
	"mint": {}, "sky": {},
}

// CheckPassword Check whether the password is correct
//
// Returns the user's UUID and true if the password is correct; otherwise returns an empty string and false
func CheckPassword(username, passwd string) (uuid string, success bool) {
	db := dbcore.GetDBInstance()
	var user models.User
	result := db.Where("username = ?", username).First(&user)
	if result.Error != nil {
		// Handle errors silently without displaying logs
		return "", false
	}
	if strings.HasPrefix(user.Passwd, "$2") {
		if bcrypt.CompareHashAndPassword([]byte(user.Passwd), []byte(legacyHash(passwd))) != nil {
			return "", false
		}
	} else {
		if subtle.ConstantTimeCompare([]byte(legacyHash(passwd)), []byte(user.Passwd)) != 1 {
			return "", false
		}
		if err := db.Model(&models.User{}).Where("uuid = ? AND passwd = ?", user.UUID, user.Passwd).Update("passwd", hashPasswd(passwd)).Error; err != nil {
			return "", false
		}
	}
	return user.UUID, true
}

// ForceResetPassword Forcefully reset user password
func ForceResetPassword(username, passwd string) (err error) {
	db := dbcore.GetDBInstance()
	result := db.Model(&models.User{}).Where("username = ?", username).Update("passwd", hashPasswd(passwd))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("Username not found")
	}
	return nil
}

// hashPasswd salts a password and hashes it
func hashPasswd(passwd string) string {
	hashed, err := bcrypt.GenerateFromPassword([]byte(legacyHash(passwd)), 12)
	if err != nil {
		panic("password hashing failed")
	}
	return string(hashed)
}

func legacyHash(passwd string) string {
	saltedPassword := passwd + constantSalt
	hash := sha256.New()
	hash.Write([]byte(saltedPassword))
	hashedPassword := base64.StdEncoding.EncodeToString(hash.Sum(nil))
	return hashedPassword
}

func CreateAccount(username, passwd string) (user models.User, err error) {
	return CreateAccountWithDB(dbcore.GetDBInstance(), username, passwd)
}

func CreateAccountWithDB(db *gorm.DB, username, passwd string) (user models.User, err error) {
	hashedPassword := hashPasswd(passwd)
	user = models.User{
		UUID:     uuid.New().String(),
		Username: username,
		Passwd:   hashedPassword,
	}
	err = db.Create(&user).Error
	if err != nil {
		return models.User{}, err
	}
	return user, nil
}

func DeleteAccountByUsername(username string) (err error) {
	return DeleteAccountByUsernameWithDB(dbcore.GetDBInstance(), username)
}

func DeleteAccountByUsernameWithDB(db *gorm.DB, username string) (err error) {
	err = db.Where("username = ?", username).Delete(&models.User{}).Error
	if err != nil {
		return err
	}
	return nil
}

func GetUserByUUID(uuid string) (user models.User, err error) {
	db := dbcore.GetDBInstance()
	err = db.Where("uuid = ?", uuid).First(&user).Error
	if err != nil {
		return models.User{}, err
	}
	return user, nil
}

// Get users via SSO messages
func GetUserBySSO(ssoID string) (user models.User, err error) {
	db := dbcore.GetDBInstance()

	// First try to find an existing user
	err = db.Where("sso_id = ?", ssoID).First(&user).Error
	if err == nil {
		return user, nil
	}

	// If the user cannot be found, return an explicit error message
	return models.User{}, fmt.Errorf("User not found: %s", ssoID)
}

func BindingExternalAccount(uuid string, sso_id string) error {
	db := dbcore.GetDBInstance()
	err := db.Model(&models.User{}).Where("uuid = ?", uuid).Update("sso_id", sso_id).Error
	if err != nil {
		return err
	}
	return nil
}

func UnbindExternalAccount(uuid string) error {
	db := dbcore.GetDBInstance()
	err := db.Model(&models.User{}).Where("uuid = ?", uuid).Update("sso_id", "").Error
	if err != nil {
		return err
	}
	return nil
}

func UpdateUser(uuid string, name, password, sso_type *string) error {
	db := dbcore.GetDBInstance()
	// Check if user exists
	var existingUser models.User
	result := db.Where("uuid = ?", uuid).First(&existingUser)
	if result.Error != nil {
		return fmt.Errorf("user not found: %s", uuid)
	}
	updates := make(map[string]interface{})
	if name != nil {
		updates["username"] = *name
	}
	if password != nil {
		updates["passwd"] = hashPasswd(*password)
	}
	if sso_type != nil {
		updates["sso_type"] = *sso_type
	}
	updates["updated_at"] = time.Now().UTC()
	err := db.Model(&models.User{}).Where("uuid = ?", uuid).Updates(updates).Error
	if err != nil {
		return err
	}
	if password != nil {
		DeleteAllSessions()
	}
	return nil
}

// UpdateUserPreferences updates only the UI preferences owned by one account.
func UpdateUserPreferences(uuid string, language, color *string) error {
	return UpdateUserPreferencesWithDB(dbcore.GetDBInstance(), uuid, language, color)
}

func UpdateUserPreferencesWithDB(db *gorm.DB, uuid string, language, color *string) error {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" {
		return fmt.Errorf("user UUID is required")
	}

	updates := make(map[string]interface{}, 3)
	if language != nil {
		normalized := strings.TrimSpace(*language)
		if _, ok := allowedPreferenceLanguages[normalized]; !ok {
			return fmt.Errorf("unsupported language preference")
		}
		updates["language"] = normalized
	}
	if color != nil {
		normalized := strings.TrimSpace(*color)
		if _, ok := allowedPreferenceColors[normalized]; !ok {
			return fmt.Errorf("unsupported color preference")
		}
		updates["color"] = normalized
	}
	if len(updates) == 0 {
		return fmt.Errorf("at least one preference is required")
	}
	updates["updated_at"] = time.Now().UTC()

	result := db.Model(&models.User{}).Where("uuid = ?", uuid).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found: %s", uuid)
	}
	return nil
}
