package jwt

import (
	"errors"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID  uuid.UUID `json:"uid"`
	IsAdmin bool      `json:"adm"`
	gojwt.RegisteredClaims
}

type Manager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewManager(secret string, accessTTLHours int) *Manager {
	return &Manager{
		secret:     []byte(secret),
		accessTTL:  time.Duration(accessTTLHours) * time.Hour,
		refreshTTL: 30 * 24 * time.Hour,
	}
}

func (m *Manager) Generate(userID uuid.UUID, isAdmin bool) (string, error) {
	claims := &Claims{
		UserID:  userID,
		IsAdmin: isAdmin,
		RegisteredClaims: gojwt.RegisteredClaims{
			ExpiresAt: gojwt.NewNumericDate(time.Now().Add(m.accessTTL)),
			IssuedAt:  gojwt.NewNumericDate(time.Now()),
			ID:        uuid.New().String(),
		},
	}
	token := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// GenerateRefresh generates a long-lived refresh token (30 days).
// Returns (tokenString, jti, error). The jti should be stored in the
// refresh-token allowlist so it can be validated and revoked (H-8).
func (m *Manager) GenerateRefresh(userID uuid.UUID, isAdmin bool) (tokenStr, jti string, err error) {
	jti = uuid.New().String()
	claims := &Claims{
		UserID:  userID,
		IsAdmin: isAdmin,
		RegisteredClaims: gojwt.RegisteredClaims{
			ExpiresAt: gojwt.NewNumericDate(time.Now().Add(m.refreshTTL)),
			IssuedAt:  gojwt.NewNumericDate(time.Now()),
			ID:        jti,
			Subject:   "refresh",
		},
	}
	token := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims)
	tokenStr, err = token.SignedString(m.secret)
	return tokenStr, jti, err
}

// RefreshTTL returns the refresh token lifetime (for Redis registration).
func (m *Manager) RefreshTTL() time.Duration {
	return m.refreshTTL
}

// AccessTTL returns the access token lifetime (for cookie MaxAge).
func (m *Manager) AccessTTL() time.Duration {
	return m.accessTTL
}

func (m *Manager) Parse(tokenStr string) (*Claims, error) {
	token, err := gojwt.ParseWithClaims(tokenStr, &Claims{}, func(t *gojwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*gojwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// ParseRefresh validates a refresh token and returns its claims.
func (m *Manager) ParseRefresh(tokenStr string) (*Claims, error) {
	claims, err := m.Parse(tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.Subject != "refresh" {
		return nil, errors.New("not a refresh token")
	}
	return claims, nil
}
