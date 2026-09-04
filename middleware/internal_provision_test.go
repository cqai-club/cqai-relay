package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestInternalProvisionAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousToken, previousSource := common.GetInternalProvisionToken()
	common.SetInternalProvisionToken("server-only-secret", "test")
	t.Cleanup(func() { common.SetInternalProvisionToken(previousToken, previousSource) })
	router := gin.New()
	router.POST("/provision", InternalProvisionAuth(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	tests := []struct {
		name          string
		authorization string
		wantStatus    int
	}{
		{name: "missing", wantStatus: http.StatusUnauthorized},
		{name: "wrong", authorization: "Bearer wrong", wantStatus: http.StatusUnauthorized},
		{name: "valid", authorization: "Bearer server-only-secret", wantStatus: http.StatusNoContent},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/provision", nil)
			if test.authorization != "" {
				request.Header.Set("Authorization", test.authorization)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			assert.Equal(t, test.wantStatus, response.Code)
		})
	}
}

func TestInternalProvisionAuthFailsClosedWithoutConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousToken, previousSource := common.GetInternalProvisionToken()
	common.SetInternalProvisionToken("", "test")
	t.Cleanup(func() { common.SetInternalProvisionToken(previousToken, previousSource) })
	router := gin.New()
	router.POST("/provision", InternalProvisionAuth(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodPost, "/provision", nil)
	request.Header.Set("Authorization", "Bearer anything")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusServiceUnavailable, response.Code)
}
