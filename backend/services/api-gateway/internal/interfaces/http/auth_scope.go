package http

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const apiTokenType = "api_token"

var (
	errAPITokenInvalid     = errors.New("invalid api token claims")
	errAPITokenScopeDenied = errors.New("api token scope denied")
)

func parseAPITokenClaims(claims jwt.MapClaims) (string, map[string]bool, error) {
	value, present := claims["token_type"]
	if !present {
		if _, hasScopes := claims["scopes"]; hasScopes {
			return "", nil, errAPITokenInvalid
		}
		return "", nil, nil
	}
	tokenType, ok := value.(string)
	if !ok || strings.TrimSpace(tokenType) != apiTokenType {
		return "", nil, errAPITokenInvalid
	}
	if strings.TrimSpace(sessionIDClaim(claims)) == "" {
		return "", nil, errAPITokenInvalid
	}
	expiresAt, err := claims.GetExpirationTime()
	if err != nil || expiresAt == nil || !expiresAt.After(time.Now()) {
		return "", nil, errAPITokenInvalid
	}
	if _, ok := credentialVersionClaimValue(claims); !ok {
		return "", nil, errAPITokenInvalid
	}
	scopes, err := parseAPITokenScopes(claims["scopes"])
	if err != nil {
		return "", nil, err
	}
	return apiTokenType, scopes, nil
}

func parseAPITokenScopes(value any) (map[string]bool, error) {
	scopes := map[string]bool{}
	switch values := value.(type) {
	case []any:
		for _, item := range values {
			scope, ok := item.(string)
			if !ok {
				return nil, errAPITokenInvalid
			}
			scopes[strings.TrimSpace(strings.ToLower(scope))] = true
		}
	case []string:
		for _, scope := range values {
			scopes[strings.TrimSpace(strings.ToLower(scope))] = true
		}
	default:
		return nil, errAPITokenInvalid
	}
	if len(scopes) == 0 {
		return nil, errAPITokenInvalid
	}
	for scope := range scopes {
		if scope != "read" && scope != "write" {
			return nil, errAPITokenInvalid
		}
	}
	return scopes, nil
}

func authorizeAPITokenScope(c *gin.Context, tokenType string, scopes map[string]bool, required string) error {
	if tokenType != apiTokenType {
		return nil
	}
	if required == "" {
		required = "write"
		if c != nil && c.Request != nil {
			switch c.Request.Method {
			case "GET", "HEAD", "OPTIONS":
				required = "read"
			}
		}
	}
	if !scopes[required] {
		return fmt.Errorf("%w: %s scope required", errAPITokenScopeDenied, required)
	}
	return nil
}

func (h *Handler) requireInteractiveAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentAuthTokenType(c) == apiTokenType {
			writeError(c, 403, "interactive session required", "permission_denied")
			c.Abort()
			return
		}
		c.Next()
	}
}

func currentAuthTokenType(c *gin.Context) string {
	value, _ := c.Get("auth_token_type")
	tokenType, _ := value.(string)
	return tokenType
}
