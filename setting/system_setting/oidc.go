package system_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

type OIDCSettings struct {
	Enabled               bool   `json:"enabled"`
	DisplayName           string `json:"display_name"`
	ClientId              string `json:"client_id"`
	ClientSecret          string `json:"client_secret"`
	WellKnown             string `json:"well_known"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserInfoEndpoint      string `json:"user_info_endpoint"`
	RedirectURI           string `json:"redirect_uri"`
	Resource              string `json:"resource"`
	Scope                 string `json:"scope"`
	AdminScope            string `json:"admin_scope"`
	RootScope             string `json:"root_scope"`
}

// 默认配置
var defaultOIDCSettings = OIDCSettings{
	Scope:      "openid profile email",
	AdminScope: "account:admin",
	RootScope:  "account:root",
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("oidc", &defaultOIDCSettings)
}

func GetOIDCSettings() *OIDCSettings {
	return &defaultOIDCSettings
}

// GetEffectiveDisplayName returns the admin-configured display name, or the
// literal "OIDC" when none has been set. Centralizing this fallback keeps the
// default in one place for both the OAuth provider name and the public
// status payload.
func (s *OIDCSettings) GetEffectiveDisplayName() string {
	if trimmed := strings.TrimSpace(s.DisplayName); trimmed != "" {
		return trimmed
	}
	return "OIDC"
}

// GetEffectiveScope returns the configured OIDC scopes and, when an API
// resource is configured, ensures the role scopes are requested as well.
// Logto only returns resource permissions when the authorization request names
// both the resource indicator and the corresponding scopes.
func (s *OIDCSettings) GetEffectiveScope() string {
	scopes := strings.Fields(strings.TrimSpace(s.Scope))
	if len(scopes) == 0 {
		scopes = strings.Fields("openid profile email")
	}
	if strings.TrimSpace(s.Resource) == "" {
		return strings.Join(scopes, " ")
	}

	seen := make(map[string]struct{}, len(scopes)+2)
	for _, scope := range scopes {
		seen[scope] = struct{}{}
	}
	for _, roleScope := range []string{strings.TrimSpace(s.AdminScope), strings.TrimSpace(s.RootScope)} {
		if roleScope == "" {
			continue
		}
		if _, ok := seen[roleScope]; ok {
			continue
		}
		scopes = append(scopes, roleScope)
		seen[roleScope] = struct{}{}
	}
	return strings.Join(scopes, " ")
}
