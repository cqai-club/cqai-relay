package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const internalProvisionTokenPurpose = "internal-provision-token"

func UpdateInternalProvisionToken(token string) error {
	token = strings.TrimSpace(token)
	encrypted, err := common.EncryptSensitiveValue(token, internalProvisionTokenPurpose)
	if err != nil {
		return err
	}
	return UpdateOption(common.InternalProvisionTokenOptionKey, encrypted)
}

func applyInternalProvisionTokenOption(encrypted string) error {
	// A persisted value is authoritative. Fail closed if it cannot be decrypted
	// instead of silently falling back to a potentially stale environment token.
	common.SetInternalProvisionToken("", "database")
	token, err := common.DecryptSensitiveValue(encrypted, internalProvisionTokenPurpose)
	if err != nil {
		return fmt.Errorf("decrypt internal provisioning token: %w", err)
	}
	common.SetInternalProvisionToken(token, "database")
	return nil
}
