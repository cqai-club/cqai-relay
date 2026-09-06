package oauth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/golang-jwt/jwt/v5"
)

const oidcMetadataCacheTTL = 5 * time.Minute

type oidcProviderMetadata struct {
	Issuer  string `json:"issuer"`
	JWKSURI string `json:"jwks_uri"`
}

type oidcJSONWebKeySet struct {
	Keys []oidcJSONWebKey `json:"keys"`
}

type oidcJSONWebKey struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type cachedOIDCMetadata struct {
	metadata  oidcProviderMetadata
	keySet    oidcJSONWebKeySet
	expiresAt time.Time
}

var oidcMetadataCache = struct {
	sync.Mutex
	items map[string]cachedOIDCMetadata
}{
	items: make(map[string]cachedOIDCMetadata),
}

func validateOIDCIDToken(ctx context.Context, rawIDToken string, settings *system_setting.OIDCSettings) (jwt.MapClaims, error) {
	if strings.TrimSpace(rawIDToken) == "" {
		return nil, errors.New("OIDC response did not include an ID token")
	}

	parser := jwt.NewParser()
	unverified := jwt.MapClaims{}
	parsed, _, err := parser.ParseUnverified(rawIDToken, unverified)
	if err != nil {
		return nil, fmt.Errorf("parse OIDC ID token header: %w", err)
	}

	algorithm, ok := parsed.Header["alg"].(string)
	if !ok || strings.TrimSpace(algorithm) == "" || algorithm == jwt.SigningMethodNone.Alg() {
		return nil, errors.New("OIDC ID token has an unsupported signing algorithm")
	}
	kid, ok := parsed.Header["kid"].(string)
	if !ok || strings.TrimSpace(kid) == "" {
		return nil, errors.New("OIDC ID token is missing a key id")
	}

	metadata, keySet, err := getOIDCMetadataAndKeys(ctx, settings.WellKnown)
	if err != nil {
		return nil, err
	}
	if metadata.Issuer == "" || metadata.JWKSURI == "" {
		return nil, errors.New("OIDC discovery metadata is missing issuer or jwks_uri")
	}

	var signingKey any
	for _, key := range keySet.Keys {
		if key.Kid != kid {
			continue
		}
		if key.Alg != "" && key.Alg != algorithm {
			return nil, fmt.Errorf("OIDC ID token key algorithm mismatch: kid=%s", kid)
		}
		signingKey, err = oidcJWKPublicKey(key)
		if err != nil {
			return nil, err
		}
		break
	}
	if signingKey == nil {
		return nil, fmt.Errorf("OIDC ID token signing key not found: kid=%s", kid)
	}

	claims := jwt.MapClaims{}
	verified, err := jwt.ParseWithClaims(
		rawIDToken,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != algorithm {
				return nil, errors.New("OIDC ID token signing method changed during validation")
			}
			return signingKey, nil
		},
		jwt.WithValidMethods([]string{algorithm}),
		jwt.WithIssuer(metadata.Issuer),
		jwt.WithAudience(settings.ClientId),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(5*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("validate OIDC ID token: %w", err)
	}
	if verified == nil || !verified.Valid {
		return nil, errors.New("OIDC ID token is invalid")
	}
	return claims, nil
}

func getOIDCMetadataAndKeys(ctx context.Context, wellKnownURL string) (oidcProviderMetadata, oidcJSONWebKeySet, error) {
	wellKnownURL = strings.TrimSpace(wellKnownURL)
	if wellKnownURL == "" {
		return oidcProviderMetadata{}, oidcJSONWebKeySet{}, errors.New("OIDC well-known URL is not configured")
	}

	now := time.Now()
	oidcMetadataCache.Lock()
	if cached, ok := oidcMetadataCache.items[wellKnownURL]; ok && now.Before(cached.expiresAt) {
		oidcMetadataCache.Unlock()
		return cached.metadata, cached.keySet, nil
	}
	oidcMetadataCache.Unlock()

	client := &http.Client{Timeout: 5 * time.Second}
	metadata, err := fetchOIDCJSON[oidcProviderMetadata](ctx, client, wellKnownURL)
	if err != nil {
		return oidcProviderMetadata{}, oidcJSONWebKeySet{}, fmt.Errorf("fetch OIDC discovery metadata: %w", err)
	}
	if metadata.JWKSURI == "" {
		return oidcProviderMetadata{}, oidcJSONWebKeySet{}, errors.New("OIDC discovery metadata is missing jwks_uri")
	}
	keySet, err := fetchOIDCJSON[oidcJSONWebKeySet](ctx, client, metadata.JWKSURI)
	if err != nil {
		return oidcProviderMetadata{}, oidcJSONWebKeySet{}, fmt.Errorf("fetch OIDC JWKS: %w", err)
	}

	oidcMetadataCache.Lock()
	oidcMetadataCache.items[wellKnownURL] = cachedOIDCMetadata{
		metadata:  metadata,
		keySet:    keySet,
		expiresAt: now.Add(oidcMetadataCacheTTL),
	}
	oidcMetadataCache.Unlock()
	return metadata, keySet, nil
}

func fetchOIDCJSON[T any](ctx context.Context, client *http.Client, endpoint string) (T, error) {
	var result T
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return result, err
	}
	req.Header.Set("Accept", "application/json")
	response, err := client.Do(req)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return result, fmt.Errorf("unexpected HTTP status %d", response.StatusCode)
	}
	if err := common.DecodeJson(response.Body, &result); err != nil {
		return result, err
	}
	return result, nil
}

func oidcJWKPublicKey(key oidcJSONWebKey) (any, error) {
	switch key.Kty {
	case "EC":
		if key.Crv != "P-384" {
			return nil, fmt.Errorf("unsupported OIDC EC curve: %s", key.Crv)
		}
		x, err := decodeOIDCBase64URL(key.X)
		if err != nil {
			return nil, fmt.Errorf("decode OIDC EC x coordinate: %w", err)
		}
		y, err := decodeOIDCBase64URL(key.Y)
		if err != nil {
			return nil, fmt.Errorf("decode OIDC EC y coordinate: %w", err)
		}
		publicKey := &ecdsa.PublicKey{Curve: elliptic.P384(), X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}
		if !publicKey.Curve.IsOnCurve(publicKey.X, publicKey.Y) {
			return nil, errors.New("OIDC EC public key is not on its curve")
		}
		return publicKey, nil
	case "RSA":
		modulus, err := decodeOIDCBase64URL(key.N)
		if err != nil {
			return nil, fmt.Errorf("decode OIDC RSA modulus: %w", err)
		}
		exponent, err := decodeOIDCBase64URL(key.E)
		if err != nil {
			return nil, fmt.Errorf("decode OIDC RSA exponent: %w", err)
		}
		if len(exponent) == 0 {
			return nil, errors.New("OIDC RSA key has an empty exponent")
		}
		var publicExponent uint64
		for _, value := range exponent {
			publicExponent = publicExponent<<8 | uint64(value)
		}
		if publicExponent == 0 || publicExponent > uint64(^uint32(0)) {
			return nil, errors.New("OIDC RSA key has an invalid exponent")
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(modulus), E: int(publicExponent)}, nil
	default:
		return nil, fmt.Errorf("unsupported OIDC key type: %s", key.Kty)
	}
}

func decodeOIDCBase64URL(value string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(value)
}

func oidcUserFromIDToken(claims jwt.MapClaims, token *OAuthToken, settings *system_setting.OIDCSettings) (*OAuthUser, error) {
	subject, _ := claims["sub"].(string)
	if strings.TrimSpace(subject) == "" {
		return nil, errors.New("OIDC ID token is missing sub")
	}

	username := firstOIDCStringClaim(claims, "preferred_username", "username")
	displayName, _ := claims["name"].(string)
	email, _ := claims["email"].(string)
	role := roleFromToken(token, settings)
	if scope, ok := claims["scope"].(string); ok && strings.TrimSpace(scope) != "" {
		role = roleFromScope(scope, settings)
	}
	return &OAuthUser{
		ProviderUserID: subject,
		Username:       username,
		DisplayName:    displayName,
		Email:          email,
		Extra: map[string]any{
			"role": role,
		},
	}, nil
}

func firstOIDCStringClaim(claims jwt.MapClaims, keys ...string) string {
	for _, key := range keys {
		if value, ok := claims[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func logOIDCIDTokenValidationFailure(ctx context.Context, err error) {
	logger.LogError(ctx, fmt.Sprintf("[OAuth-OIDC] ID token validation failed: %s", err.Error()))
}
