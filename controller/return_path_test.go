package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
)

func TestPaymentReturnPathUsesDefaultDashboardRoutes(t *testing.T) {
	previousAddress := system_setting.ServerAddress
	system_setting.ServerAddress = "https://dashboard.example.com/"
	t.Cleanup(func() { system_setting.ServerAddress = previousAddress })

	assert.Equal(
		t,
		"https://dashboard.example.com/wallet?pay=success",
		paymentReturnPath("/wallet?pay=success"),
	)
	assert.Equal(
		t,
		"https://dashboard.example.com/usage-logs",
		paymentReturnPath("/usage-logs"),
	)
}

func TestPaymentReturnURLAddsOrderIDForWebAndDesktopClients(t *testing.T) {
	webURL, err := addPaymentReturnParam("https://app.example.com/billing/result", "cqai_order_id", "ref_123")
	assert.NoError(t, err)
	assert.Equal(t, "https://app.example.com/billing/result?cqai_order_id=ref_123", webURL)

	desktopURL, err := addPaymentReturnParam("cqai://payment/result?status=success", "cqai_order_id", "ref_123")
	assert.NoError(t, err)
	assert.Equal(t, "cqai://payment/result?cqai_order_id=ref_123&status=success", desktopURL)
}
