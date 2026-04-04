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
	secret        []byte
	accessTTL     time.Duration
}

func NewManager(secret string, accessTTLHours int) *Manager {
	return &Manager{
		secret:    []byte(secret),
		accessTTL: time.Duration(accessTTLHours) * time.Hour,
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
