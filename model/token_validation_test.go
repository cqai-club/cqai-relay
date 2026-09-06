package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateUserTokenDistinguishesExhaustedQuotaFromInvalidToken(t *testing.T) {
	truncateTables(t)
	common.RedisEnabled = false

	enabledWithNoQuota := &Token{
		UserId:      1,
		Key:         "enabled-no-quota",
		Status:      common.TokenStatusEnabled,
		ExpiredTime: -1,
		RemainQuota: 0,
	}
	require.NoError(t, enabledWithNoQuota.Insert())

	validated, err := ValidateUserToken(enabledWithNoQuota.Key)
	require.NotNil(t, validated)
	assert.Equal(t, enabledWithNoQuota.Id, validated.Id)
	assert.ErrorIs(t, err, ErrTokenQuotaExhausted)

	var persisted Token
	require.NoError(t, DB.First(&persisted, enabledWithNoQuota.Id).Error)
	assert.Equal(t, common.TokenStatusExhausted, persisted.Status)

	_, err = ValidateUserToken(enabledWithNoQuota.Key)
	assert.ErrorIs(t, err, ErrTokenQuotaExhausted)

	invalidStatus := &Token{
		UserId:      1,
		Key:         "disabled-token",
		Status:      common.TokenStatusDisabled,
		ExpiredTime: -1,
		RemainQuota: 100,
	}
	require.NoError(t, invalidStatus.Insert())
	_, err = ValidateUserToken(invalidStatus.Key)
	assert.ErrorIs(t, err, ErrTokenInvalid)

	unlimited := &Token{
		UserId:         1,
		Key:            "unlimited-no-quota",
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		RemainQuota:    0,
		UnlimitedQuota: true,
	}
	require.NoError(t, unlimited.Insert())
	_, err = ValidateUserToken(unlimited.Key)
	require.NoError(t, err)
}
