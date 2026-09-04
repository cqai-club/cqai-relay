package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const (
	internalProvisionTokenBytes     = 32
	internalProvisionTokenMinLength = 32
	internalProvisionTokenMaxLength = 512
)

type internalProvisionTokenUpdateRequest struct {
	Token string `json:"token"`
}

func GetInternalProvisionTokenStatus(c *gin.Context) {
	token, source := common.GetInternalProvisionToken()
	common.ApiSuccess(c, gin.H{
		"configured":       token != "",
		"source":           source,
		"encryption_ready": common.CryptoSecretPersistent,
	})
}

func GenerateInternalProvisionToken(c *gin.Context) {
	token, err := common.GenerateSecureToken(internalProvisionTokenBytes)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"token": token})
}

func RevealInternalProvisionToken(c *gin.Context) {
	token, _ := common.GetInternalProvisionToken()
	if token == "" {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Internal provisioning token is not configured",
		})
		return
	}
	recordManageAudit(c, "option.view", map[string]interface{}{
		"key": common.InternalProvisionTokenOptionKey,
	})
	common.ApiSuccess(c, gin.H{"token": token})
}

func UpdateInternalProvisionToken(c *gin.Context) {
	if !common.CryptoSecretPersistent {
		common.ApiErrorMsg(c, "Set a stable SESSION_SECRET or CRYPTO_SECRET before saving encrypted credentials")
		return
	}
	var request internalProvisionTokenUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorMsg(c, "Invalid internal provisioning token request")
		return
	}
	token := strings.TrimSpace(request.Token)
	if token != request.Token {
		common.ApiErrorMsg(c, "Internal provisioning token cannot contain whitespace")
		return
	}
	if len(token) < internalProvisionTokenMinLength || len(token) > internalProvisionTokenMaxLength {
		common.ApiErrorMsg(c, "Internal provisioning token must be between 32 and 512 characters")
		return
	}
	if strings.ContainsAny(token, " \t\r\n") {
		common.ApiErrorMsg(c, "Internal provisioning token cannot contain whitespace")
		return
	}
	if err := model.UpdateInternalProvisionToken(token); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "option.update", map[string]interface{}{
		"key": common.InternalProvisionTokenOptionKey,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}
