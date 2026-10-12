package auth

import (
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// SessionTokenUserID returns the user id of a valid Paper session JWT
// (same issuer / audience / signature rules as the auth middleware;
// pending-purpose tokens are not sessions). ok is false for anything
// else, including an expired or tampered token.
func SessionTokenUserID(tokenStr, secret string) (uint, bool) {
	if tokenStr == "" || secret == "" {
		return 0, false
	}
	tok, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	}, jwt.WithIssuer(JWTIssuer), jwt.WithAudience(JWTAudienceAPI))
	if err != nil || !tok.Valid {
		return 0, false
	}
	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return 0, false
	}
	if jti, _ := claims["jti"].(string); strings.HasPrefix(jti, "purpose:") {
		return 0, false
	}
	id, ok := claims["id"].(float64)
	if !ok || id <= 0 {
		return 0, false
	}
	return uint(id), true
}
