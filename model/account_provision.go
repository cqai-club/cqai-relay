package model

import (
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrAccountProvisionConflict = errors.New("account provisioning conflict")
	ErrExternalAccountCollision = errors.New("external account identity collision")
	ErrExternalAccountDisabled  = errors.New("external account is disabled")
	ErrAppCredentialUnavailable = errors.New("app credential is unavailable")
)

// ExternalAccountIdentity binds one external issuer/subject pair to one
// NewAPI user. IdentityKey is a SHA-256 digest so the unique index remains
// portable across SQLite, MySQL, and PostgreSQL.
type ExternalAccountIdentity struct {
	Id          int64     `json:"id" gorm:"primaryKey"`
	IdentityKey string    `json:"-" gorm:"type:char(64);not null;uniqueIndex"`
	Issuer      string    `json:"issuer" gorm:"type:varchar(512);not null"`
	Subject     string    `json:"subject" gorm:"type:varchar(255);not null"`
	UserId      int       `json:"user_id" gorm:"not null;index"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (ExternalAccountIdentity) TableName() string {
	return "external_account_identities"
}

// AppCredential assigns one durable NewAPI token to each user/application
// pair. The token is intentionally not recreated after administrative removal.
type AppCredential struct {
	Id        int64     `json:"id" gorm:"primaryKey"`
	UserId    int       `json:"user_id" gorm:"not null;uniqueIndex:idx_app_credential_user_platform,priority:1"`
	Platform  string    `json:"platform" gorm:"type:varchar(64);not null;uniqueIndex:idx_app_credential_user_platform,priority:2"`
	TokenId   int       `json:"token_id" gorm:"not null;uniqueIndex"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AppCredential) TableName() string {
	return "app_credentials"
}

type AccountProvisionInput struct {
	IdentityKey string
	Issuer      string
	Subject     string
	Platform    string
	Username    string
	Password    string
	DisplayName string
	Email       string
	Role        int
	TokenKey    string
	TokenQuota  int
}

type AccountProvisionResult struct {
	User              User
	Token             Token
	CredentialId      int64
	UserCreated       bool
	CredentialCreated bool
}

// ProvisionExternalAccount atomically creates or retrieves the user and
// application token for an external identity. Unique indexes provide the
// final concurrency guard; a losing transaction reads back the winner.
func ProvisionExternalAccount(input AccountProvisionInput) (*AccountProvisionResult, error) {
	result := &AccountProvisionResult{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		identity := ExternalAccountIdentity{}
		err := lockForUpdate(tx).Where("identity_key = ?", input.IdentityKey).First(&identity).Error
		switch {
		case err == nil:
			if identity.Issuer != input.Issuer || identity.Subject != input.Subject {
				return ErrExternalAccountCollision
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			user := User{
				Username:    input.Username,
				Password:    input.Password,
				DisplayName: input.DisplayName,
				Email:       input.Email,
				Role:        input.Role,
				Status:      common.UserStatusEnabled,
				Group:       "default",
			}
			if user.Email != "" {
				available, checkErr := IsEmailAvailableWithTx(tx, user.Email, 0)
				if checkErr != nil {
					return checkErr
				}
				if !available {
					user.Email = ""
				}
			}
			if err := user.InsertWithTx(tx, 0); err != nil {
				return err
			}
			identity = ExternalAccountIdentity{
				IdentityKey: input.IdentityKey,
				Issuer:      input.Issuer,
				Subject:     input.Subject,
				UserId:      user.Id,
			}
			created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&identity)
			if created.Error != nil {
				return created.Error
			}
			if created.RowsAffected == 0 {
				return ErrAccountProvisionConflict
			}
			result.User = user
			result.UserCreated = true
		default:
			return err
		}

		if !result.UserCreated {
			if err := tx.Unscoped().First(&result.User, identity.UserId).Error; err != nil {
				return err
			}
			if result.User.DeletedAt.Valid || result.User.Status != common.UserStatusEnabled {
				return ErrExternalAccountDisabled
			}
		}

		credential := AppCredential{}
		err = lockForUpdate(tx).
			Where("user_id = ? AND platform = ?", result.User.Id, input.Platform).
			First(&credential).Error
		switch {
		case err == nil:
			if err := tx.First(&result.Token, credential.TokenId).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrAppCredentialUnavailable
				}
				return err
			}
			result.CredentialId = credential.Id
			return nil
		case !errors.Is(err, gorm.ErrRecordNotFound):
			return err
		}

		now := common.GetTimestamp()
		result.Token = Token{
			UserId:         result.User.Id,
			Key:            input.TokenKey,
			Status:         common.TokenStatusEnabled,
			Name:           "account:" + input.Platform,
			CreatedTime:    now,
			AccessedTime:   now,
			ExpiredTime:    -1,
			RemainQuota:    input.TokenQuota,
			UnlimitedQuota: false,
		}
		if err := tx.Create(&result.Token).Error; err != nil {
			return err
		}
		credential = AppCredential{
			UserId:   result.User.Id,
			Platform: input.Platform,
			TokenId:  result.Token.Id,
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&credential)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			return ErrAccountProvisionConflict
		}
		result.CredentialId = credential.Id
		result.CredentialCreated = true
		return nil
	})
	if errors.Is(err, ErrAccountProvisionConflict) {
		return loadProvisionedExternalAccount(input)
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

func loadProvisionedExternalAccount(input AccountProvisionInput) (*AccountProvisionResult, error) {
	identity := ExternalAccountIdentity{}
	if err := DB.Where("identity_key = ?", input.IdentityKey).First(&identity).Error; err != nil {
		return nil, err
	}
	if identity.Issuer != input.Issuer || identity.Subject != input.Subject {
		return nil, ErrExternalAccountCollision
	}
	result := &AccountProvisionResult{}
	if err := DB.Unscoped().First(&result.User, identity.UserId).Error; err != nil {
		return nil, err
	}
	if result.User.DeletedAt.Valid || result.User.Status != common.UserStatusEnabled {
		return nil, ErrExternalAccountDisabled
	}
	credential := AppCredential{}
	if err := DB.Where("user_id = ? AND platform = ?", result.User.Id, input.Platform).First(&credential).Error; err != nil {
		return nil, err
	}
	if err := DB.First(&result.Token, credential.TokenId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAppCredentialUnavailable
		}
		return nil, err
	}
	result.CredentialId = credential.Id
	return result, nil
}
