package service

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

var (
	ErrInvalidAccountProvisionRequest = errors.New("invalid account provisioning request")
	ErrProvisionedAccountUnavailable  = errors.New("provisioned account is unavailable")
	ErrProvisionedIdentityConflict    = errors.New("provisioned identity conflict")
)

var accountPlatformPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

type AccountProvisionRequest struct {
	Issuer   string `json:"issuer"`
	Subject  string `json:"subject"`
	Platform string `json:"platform"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	// Optional NewAPI numeric role supplied by the trusted bridge
	// (Account Service). Only 1 (common), 10 (admin), and 100 (root) are
	// accepted on first creation; existing users are never re-role'd.
	Role int `json:"role"`
}

type AccountProvisionResponse struct {
	UserId            int    `json:"user_id"`
	TokenId           int    `json:"token_id"`
	CredentialId      int64  `json:"credential_id"`
	ApiKey            string `json:"api_key"`
	Platform          string `json:"platform"`
	UserCreated       bool   `json:"user_created"`
	CredentialCreated bool   `json:"credential_created"`
	KeyCreated        bool   `json:"key_created"`
	UserStatus        int    `json:"user_status"`
	TokenStatus       int    `json:"token_status"`
	Quota             int    `json:"quota"`
	QuotaUsed         int    `json:"quota_used"`
}

// ProvisionAccount creates or retrieves one NewAPI user and one key per
// external identity/application pair. Role, group, and quota policy are fixed
// server-side and cannot be elevated by request fields.
func ProvisionAccount(request AccountProvisionRequest) (*AccountProvisionResponse, error) {
	issuer := strings.TrimSpace(request.Issuer)
	subject := strings.TrimSpace(request.Subject)
	platform := strings.ToLower(strings.TrimSpace(request.Platform))
	if issuer == "" || utf8.RuneCountInString(issuer) > 512 {
		return nil, fmt.Errorf("%w: issuer is required and must not exceed 512 characters", ErrInvalidAccountProvisionRequest)
	}
	parsedIssuer, err := url.Parse(issuer)
	if err != nil || parsedIssuer.Scheme == "" || parsedIssuer.Host == "" || parsedIssuer.User != nil || parsedIssuer.Fragment != "" || parsedIssuer.RawQuery != "" {
		return nil, fmt.Errorf("%w: issuer must be an absolute URL without credentials, query, or fragment", ErrInvalidAccountProvisionRequest)
	}
	if subject == "" || utf8.RuneCountInString(subject) > 255 || strings.ContainsRune(subject, '\x00') {
		return nil, fmt.Errorf("%w: subject is required and must not exceed 255 characters", ErrInvalidAccountProvisionRequest)
	}
	if !accountPlatformPattern.MatchString(platform) {
		return nil, fmt.Errorf("%w: platform must use lowercase letters, numbers, hyphens, or underscores", ErrInvalidAccountProvisionRequest)
	}

	email := model.NormalizeEmail(request.Email)
	if email != "" {
		address, parseErr := mail.ParseAddress(email)
		if utf8.RuneCountInString(email) > 50 || parseErr != nil || address.Address != email {
			return nil, fmt.Errorf("%w: email is invalid or exceeds 50 characters", ErrInvalidAccountProvisionRequest)
		}
	}
	displayName := strings.TrimSpace(request.Name)
	if runes := []rune(displayName); len(runes) > 20 {
		displayName = string(runes[:20])
	}
	if displayName == "" {
		displayName = "AI Account"
	}
	role := normalizeRole(request.Role)

	identityDigest := sha256.Sum256([]byte(issuer + "\x00" + subject))
	identityKey := fmt.Sprintf("%x", identityDigest)
	password, err := common.GenerateRandomCharsKey(20)
	if err != nil {
		return nil, err
	}
	tokenKey, err := common.GenerateKey()
	if err != nil {
		return nil, err
	}
	result, err := model.ProvisionExternalAccount(model.AccountProvisionInput{
		IdentityKey: identityKey,
		Issuer:      issuer,
		Subject:     subject,
		Platform:    platform,
		Username:    "acct_" + identityKey[:15],
		Password:    password,
		DisplayName: displayName,
		Email:       email,
		Role:        role,
		TokenKey:    tokenKey,
		// Keep the per-key quota aligned with NewAPI's configured initial user
		// quota. Never use MaxWalletQuota here: it is a wallet safety boundary,
		// not an unlimited API-key grant.
		TokenQuota: common.QuotaForNewUser,
	})
	if err != nil {
		switch {
		case errors.Is(err, model.ErrExternalAccountDisabled), errors.Is(err, model.ErrAppCredentialUnavailable):
			return nil, fmt.Errorf("%w: %v", ErrProvisionedAccountUnavailable, err)
		case errors.Is(err, model.ErrExternalAccountCollision):
			return nil, fmt.Errorf("%w: %v", ErrProvisionedIdentityConflict, err)
		default:
			return nil, err
		}
	}
	if result.UserCreated {
		result.User.FinalizeOAuthUserCreation(0)
	}

	return &AccountProvisionResponse{
		UserId:            result.User.Id,
		TokenId:           result.Token.Id,
		CredentialId:      result.CredentialId,
		ApiKey:            "sk-" + result.Token.GetFullKey(),
		Platform:          platform,
		UserCreated:       result.UserCreated,
		CredentialCreated: result.CredentialCreated,
		KeyCreated:        result.CredentialCreated,
		UserStatus:        result.User.Status,
		TokenStatus:       result.Token.Status,
		Quota:             result.User.Quota,
		QuotaUsed:         result.User.UsedQuota,
	}, nil
}

// normalizeRole restricts provisioning role to common/admin/root. 0 (or any
// unknown value) means a normal user; provisioning can never elevate beyond
// the values the trusted bridge is configured to send.
func normalizeRole(role int) int {
	if common.IsValidateRole(role) && role > common.RoleCommonUser {
		return role
	}
	return common.RoleCommonUser
}
