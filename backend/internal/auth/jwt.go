package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
	jwt.RegisteredClaims
}

type TokenIssuer struct {
	secret    []byte
	accessTTL time.Duration
}

func NewTokenIssuer(secret string, accessTTLMinutes int) *TokenIssuer {
	return &TokenIssuer{
		secret:    []byte(secret),
		accessTTL: time.Duration(accessTTLMinutes) * time.Minute,
	}
}

func (t *TokenIssuer) IssueAccess(userID, name string, roles []string) (string, time.Time, error) {
	exp := time.Now().Add(t.accessTTL)
	claims := Claims{
		Name:  name,
		Roles: roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    "eoffice-api",
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	return signed, exp, err
}

func (t *TokenIssuer) ParseAccess(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(tok *jwt.Token) (any, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("metode signing tidak dikenal: %v", tok.Header["alg"])
		}
		return t.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("token tidak valid")
	}
	return claims, nil
}

// NewRefreshToken menghasilkan token opaque acak; disimpan di Redis dengan TTL
// dan dirotasi setiap kali dipakai (lihat handler auth).
func NewRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
