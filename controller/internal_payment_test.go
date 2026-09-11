package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInternalPaymentRequiresTheProvisioningToken(t *testing.T) {
	previousToken, previousSource := common.GetInternalProvisionToken()
	common.SetInternalProvisionToken("payment-internal-secret", "test")
	t.Cleanup(func() { common.SetInternalProvisionToken(previousToken, previousSource) })

	router := gin.New()
	router.POST("/api/internal/payment/topup/:provider", middleware.InternalProvisionAuth(), InternalCreateTopUp)

	request := httptest.NewRequest(http.MethodPost, "/api/internal/payment/topup/unknown", strings.NewReader(`{"user_id":1,"payload":{}}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
	assert.Contains(t, response.Body.String(), "INVALID_INTERNAL_CREDENTIALS")
}

func TestInternalCreateTopUpRejectsMalformedEnvelope(t *testing.T) {
	router := gin.New()
	router.POST("/api/internal/payment/topup/:provider", InternalCreateTopUp)

	request := httptest.NewRequest(http.MethodPost, "/api/internal/payment/topup/stripe", strings.NewReader(`{"user_id":0,"payload":{}}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Contains(t, response.Body.String(), "INVALID_PAYMENT_REQUEST")
}

func TestInternalCreateTopUpRejectsUnknownProvider(t *testing.T) {
	router := gin.New()
	router.POST("/api/internal/payment/topup/:provider", InternalCreateTopUp)

	request := httptest.NewRequest(http.MethodPost, "/api/internal/payment/topup/unknown", strings.NewReader(`{"user_id":1,"payload":{}}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusNotFound, response.Code)
	assert.Contains(t, response.Body.String(), "PAYMENT_PROVIDER_NOT_FOUND")
}

func TestInternalPaymentPayloadMarksTrustedAccountServiceContext(t *testing.T) {
	router := gin.New()
	router.POST("/internal", func(c *gin.Context) {
		withInternalPaymentPayload(c, func(c *gin.Context) {
			assert.True(t, c.GetBool(internalPaymentRequestContextKey))
			assert.Equal(t, 42, c.GetInt("id"))
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})
	})

	request := httptest.NewRequest(http.MethodPost, "/internal", strings.NewReader(`{"user_id":42,"payload":{}}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `"ok":true`)
}

func TestInternalPaymentUserIdRejectsInvalidQuery(t *testing.T) {
	router := gin.New()
	router.GET("/api/internal/payment/topup/self", InternalGetUserTopUps)

	request := httptest.NewRequest(http.MethodGet, "/api/internal/payment/topup/self?user_id=0", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Contains(t, response.Body.String(), "INVALID_PAYMENT_USER")
}
