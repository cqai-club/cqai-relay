package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSumUsedQuotaPreservesQuotaAlongsideRecentRates(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&Log{}))

	previousDB, previousType := LOG_DB, common.LogDatabaseType()
	LOG_DB = db
	common.SetLogDatabaseType(common.DatabaseTypeSQLite)
	initCol()
	t.Cleanup(func() {
		LOG_DB = previousDB
		common.SetLogDatabaseType(previousType)
		initCol()
	})

	now := time.Now().Unix()
	require.NoError(t, db.Create(&[]Log{
		{Type: LogTypeConsume, Username: "alice", CreatedAt: now - 120, Quota: 100, PromptTokens: 10, CompletionTokens: 20},
		{Type: LogTypeConsume, Username: "alice", CreatedAt: now - 10, Quota: 200, PromptTokens: 30, CompletionTokens: 40},
		{Type: LogTypeTopup, Username: "alice", CreatedAt: now - 10, Quota: 999},
		{Type: LogTypeConsume, Username: "bob", CreatedAt: now - 10, Quota: 500, PromptTokens: 50},
	}).Error)

	stat, err := SumUsedQuota(LogTypeConsume, now-300, now, "", "alice", "", 0, "")
	require.NoError(t, err)
	assert.Equal(t, Stat{Quota: 300, Rpm: 1, Tpm: 70}, stat)

	stat, err = SumUsedQuota(LogTypeConsume, now-300, now-90, "", "alice", "", 0, "")
	require.NoError(t, err)
	assert.Equal(t, Stat{Quota: 100, Rpm: 1, Tpm: 70}, stat, "historical quota and current request rates use independent time windows")
}
