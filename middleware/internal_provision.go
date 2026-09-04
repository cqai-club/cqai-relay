package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// InternalProvisionAuth protects the server-to-server provisioning endpoint.
// The configured token is read for every request so an administrator can
// rotate it without restarting the process. Fixed-size digests keep the token
// value and length out of the comparison timing.
func InternalProvisionAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		configured, _ := common.GetInternalProvisionToken()
		if configured == "" {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"success": false,
				"code":    "INTERNAL_PROVISION_NOT_CONFIGURED",
				"message": "Internal account provisioning is not configured",
			})
			return
		}
		configuredDigest := sha256.Sum256([]byte(configured))
		authorization := strings.TrimSpace(c.GetHeader("Authorization"))
		parts := strings.Fields(authorization)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"code":    "INVALID_INTERNAL_CREDENTIALS",
				"message": "Invalid internal credentials",
			})
			return
		}
		providedDigest := sha256.Sum256([]byte(parts[1]))
		if subtle.ConstantTimeCompare(configuredDigest[:], providedDigest[:]) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"code":    "INVALID_INTERNAL_CREDENTIALS",
				"message": "Invalid internal credentials",
			})
			return
		}
		c.Next()
	}
}
