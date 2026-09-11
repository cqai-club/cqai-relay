package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
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

func TestInternalPaymentReturnURLSkipsLegacyAllowlist(t *testing.T) {
	previousDomains := constant.TrustedRedirectDomains
	previousURIs := constant.TrustedPaymentRedirectURIs
	constant.TrustedRedirectDomains = nil
	constant.TrustedPaymentRedirectURIs = nil
	t.Cleanup(func() {
		constant.TrustedRedirectDomains = previousDomains
		constant.TrustedPaymentRedirectURIs = previousURIs
	})

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Set(internalPaymentRequestContextKey, true)

	resolved, err := resolvePaymentReturnURL(context, "https://unlisted.example.com/billing/result", "")
	assert.NoError(t, err)
	assert.Equal(t, "https://unlisted.example.com/billing/result", resolved)

	resolved, err = resolvePaymentReturnURL(context, "cqai://payment/result?status=cancelled", "")
	assert.NoError(t, err)
	assert.Equal(t, "cqai://payment/result?status=cancelled", resolved)
}

func TestInternalPaymentReturnURLStillRejectsUnsafeURLs(t *testing.T) {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Set(internalPaymentRequestContextKey, true)

	_, err := resolvePaymentReturnURL(context, "javascript://alert(1)", "")
	assert.Error(t, err)
	_, err = resolvePaymentReturnURL(context, "https://user:password@example.com/result", "")
	assert.Error(t, err)
}

func TestLegacyPaymentReturnURLStillUsesTrustedAllowlist(t *testing.T) {
	previousDomains := constant.TrustedRedirectDomains
	previousURIs := constant.TrustedPaymentRedirectURIs
	constant.TrustedRedirectDomains = nil
	constant.TrustedPaymentRedirectURIs = nil
	t.Cleanup(func() {
		constant.TrustedRedirectDomains = previousDomains
		constant.TrustedPaymentRedirectURIs = previousURIs
	})

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	_, err := resolvePaymentReturnURL(context, "https://unlisted.example.com/billing/result", "")
	assert.Error(t, err)
	assert.Error(t, common.ValidatePaymentRedirectURL("https://unlisted.example.com/billing/result"))
}
