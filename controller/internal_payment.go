package controller

import (
	"bytes"
	"io"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// internalPaymentEnvelope is accepted only behind InternalProvisionAuth. The
// public Account Service resolves the Logto identity to a NewAPI user before
// sending this envelope; browsers never provide user_id directly.
type internalPaymentEnvelope struct {
	UserId  int                    `json:"user_id"`
	Payload map[string]interface{} `json:"payload"`
}

const internalPaymentRequestContextKey = "account_service_payment"

// InternalGetTopUpInfo exposes the existing server-side payment configuration
// to the trusted Account Service without exposing the NewAPI dashboard auth
// flow to a Logto browser client.
func InternalGetTopUpInfo(c *gin.Context) {
	GetTopUpInfo(c)
}

// InternalGetUserTopUps returns only the NewAPI user's own top-up history.
func InternalGetUserTopUps(c *gin.Context) {
	userId, ok := internalPaymentUserId(c)
	if !ok {
		return
	}
	c.Set("id", userId)
	GetUserTopUps(c)
}

// InternalCreateTopUp dispatches to the existing payment adapters after
// replacing the public dashboard-auth context with a trusted user id.
func InternalCreateTopUp(c *gin.Context) {
	dispatch := func(c *gin.Context) {
		switch c.Param("provider") {
		case "epay":
			RequestEpay(c)
		case "stripe":
			RequestStripePay(c)
		case "creem":
			RequestCreemPay(c)
		case "waffo":
			RequestWaffoPay(c)
		case "waffo-pancake":
			RequestWaffoPancakePay(c)
		default:
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"code":    "PAYMENT_PROVIDER_NOT_FOUND",
				"message": "Payment provider is not supported",
			})
		}
	}
	withInternalPaymentPayload(c, dispatch)
}

// InternalGetSubscriptionPlans exposes enabled plans from the Relay database.
func InternalGetSubscriptionPlans(c *gin.Context) {
	GetSubscriptionPlans(c)
}

// InternalGetSubscriptionSelf returns the subscriptions belonging to the
// trusted NewAPI user id.
func InternalGetSubscriptionSelf(c *gin.Context) {
	userId, ok := internalPaymentUserId(c)
	if !ok {
		return
	}
	c.Set("id", userId)
	GetSubscriptionSelf(c)
}

// InternalPurchaseSubscription dispatches to the existing subscription
// payment adapters using the same payment compliance and order logic as the
// native NewAPI dashboard.
func InternalPurchaseSubscription(c *gin.Context) {
	dispatch := func(c *gin.Context) {
		switch c.Param("provider") {
		case "balance":
			SubscriptionRequestBalancePay(c)
		case "epay":
			SubscriptionRequestEpay(c)
		case "stripe":
			SubscriptionRequestStripePay(c)
		case "creem":
			SubscriptionRequestCreemPay(c)
		case "waffo-pancake":
			SubscriptionRequestWaffoPancakePay(c)
		default:
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"code":    "PAYMENT_PROVIDER_NOT_FOUND",
				"message": "Payment provider is not supported",
			})
		}
	}
	withInternalPaymentPayload(c, dispatch)
}

func withInternalPaymentPayload(c *gin.Context, dispatch func(*gin.Context)) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "INVALID_PAYMENT_REQUEST",
			"message": "Invalid payment request",
		})
		return
	}

	request := internalPaymentEnvelope{}
	if err := common.Unmarshal(body, &request); err != nil || request.UserId <= 0 || request.Payload == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "INVALID_PAYMENT_REQUEST",
			"message": "Invalid payment request",
		})
		return
	}

	payload, err := common.Marshal(request.Payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "INVALID_PAYMENT_REQUEST",
			"message": "Invalid payment request",
		})
		return
	}

	c.Set("id", request.UserId)
	c.Set(internalPaymentRequestContextKey, true)
	c.Request.Body = io.NopCloser(bytes.NewReader(payload))
	dispatch(c)
}

func internalPaymentUserId(c *gin.Context) (int, bool) {
	userId, err := strconv.Atoi(c.Query("user_id"))
	if err != nil || userId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "INVALID_PAYMENT_USER",
			"message": "Invalid payment user",
		})
		return 0, false
	}
	return userId, true
}
