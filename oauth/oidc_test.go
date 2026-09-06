package oauth

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOIDCProvider_GetName(t *testing.T) {
	settings := system_setting.GetOIDCSettings()
	originalDisplayName := settings.DisplayName
	defer func() { settings.DisplayName = originalDisplayName }()

	p := &OIDCProvider{}

	settings.DisplayName = ""
	assert.Equal(t, "OIDC", p.GetName())

	settings.DisplayName = "  Acme SSO  "
	assert.Equal(t, "Acme SSO", p.GetName())
}

func TestOIDCRoleFromScope(t *testing.T) {
	settings := &system_setting.OIDCSettings{
		AdminScope: "account:admin",
		RootScope:  "account:root",
	}

	tests := []struct {
		name  string
		scope string
		want  int
	}{
		{name: "ordinary user", scope: "openid profile email", want: common.RoleCommonUser},
		{name: "admin user", scope: "openid account:admin", want: common.RoleAdminUser},
		{name: "root wins over admin", scope: "account:admin account:root", want: common.RoleRootUser},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, roleFromScope(tt.scope, settings))
		})
	}
}

func TestOIDCRoleFromTokenPrefersExchangeMetadata(t *testing.T) {
	settings := &system_setting.OIDCSettings{
		AdminScope: "account:admin",
		RootScope:  "account:root",
	}
	token := &OAuthToken{
		Scope: "openid",
		Extra: map[string]any{"role": common.RoleRootUser},
	}

	require.Equal(t, common.RoleRootUser, roleFromToken(token, settings))
}

func TestOIDCExchangeTokenUsesConfiguredRedirectAndScope(t *testing.T) {
	settings := system_setting.GetOIDCSettings()
	originalClientID := settings.ClientId
	originalClientSecret := settings.ClientSecret
	originalTokenEndpoint := settings.TokenEndpoint
	originalRedirectURI := settings.RedirectURI
	originalResource := settings.Resource
	originalScope := settings.Scope
	t.Cleanup(func() {
		settings.ClientId = originalClientID
		settings.ClientSecret = originalClientSecret
		settings.TokenEndpoint = originalTokenEndpoint
		settings.RedirectURI = originalRedirectURI
		settings.Resource = originalResource
		settings.Scope = originalScope
	})

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if !assert.NoError(t, err) {
			return
		}
		values, err := url.ParseQuery(string(body))
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, "https://relay.example.com/oauth/oidc", values.Get("redirect_uri"))
		assert.Equal(t, "https://account.example.com", values.Get("resource"))
		assert.Equal(t, "openid profile email account:admin account:root", values.Get("scope"))
		writer.Header().Set("Content-Type", "application/json")
		_, err = writer.Write([]byte(`{"access_token":"access","token_type":"Bearer","scope":"openid account:root"}`))
		assert.NoError(t, err)
	}))
	defer server.Close()

	settings.ClientId = "client-id"
	settings.ClientSecret = "client-secret"
	settings.TokenEndpoint = server.URL
	settings.RedirectURI = "https://relay.example.com/oauth/oidc"
	settings.Resource = "https://account.example.com"
	settings.Scope = "openid profile email"

	provider := &OIDCProvider{}
	token, err := provider.ExchangeToken(context.Background(), "authorization-code", nil)
	require.NoError(t, err)
	require.NotNil(t, token.Extra)
	assert.Equal(t, common.RoleRootUser, token.Extra["role"])
}
