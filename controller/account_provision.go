package controller

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func ProvisionExternalAccount(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	request := service.AccountProvisionRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "INVALID_PROVISION_REQUEST",
			"message": "Invalid provisioning request",
		})
		return
	}
	response, err := service.ProvisionAccount(request)
	if err == nil {
		common.ApiSuccess(c, response)
		return
	}
	if errors.Is(err, service.ErrInvalidAccountProvisionRequest) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "INVALID_PROVISION_REQUEST",
			"message": err.Error(),
		})
		return
	}
	if errors.Is(err, service.ErrProvisionedAccountUnavailable) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"code":    "PROVISIONED_ACCOUNT_UNAVAILABLE",
			"message": "Provisioned account is unavailable",
		})
		return
	}
	if errors.Is(err, service.ErrProvisionedIdentityConflict) {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"code":    "PROVISIONED_IDENTITY_CONFLICT",
			"message": "Provisioned identity conflicts with an existing account",
		})
		return
	}
	common.SysError("internal account provisioning failed: " + err.Error())
	c.JSON(http.StatusInternalServerError, gin.H{
		"success": false,
		"code":    "ACCOUNT_PROVISION_FAILED",
		"message": "Account provisioning failed",
	})
}
