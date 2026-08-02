package auth

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/config"
)

const (
	accessPurpose  = "access"
	refreshPurpose = "refresh"
)

var errInvalidToken = errors.New("invalid token")

type tokenClaims struct {
	Purpose     string `json:"purpose"`
	FamilyID    string `json:"family_id"`
	AuthVersion int    `json:"auth_version"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	config config.AuthConfig
	now    func() time.Time
}

func NewTokenManager(cfg config.AuthConfig) *TokenManager {
	return &TokenManager{config: cfg, now: time.Now}
}

func (m *TokenManager) issueAccess(userID, jti, familyID string, authVersion int) (string, time.Time, error) {
	return m.issue(userID, jti, familyID, authVersion, accessPurpose, m.config.AccessAudience, m.config.AccessSecret, m.config.AccessTTL)
}

func (m *TokenManager) issueRefresh(userID, jti, familyID string, authVersion int) (string, time.Time, error) {
	return m.issue(userID, jti, familyID, authVersion, refreshPurpose, m.config.RefreshAudience, m.config.RefreshSecret, m.config.RefreshTTL)
}

func (m *TokenManager) issue(userID, jti, familyID string, authVersion int, purpose, audience, secret string, ttl time.Duration) (string, time.Time, error) {
	now := m.now().UTC()
	expiresAt := now.Add(ttl)
	claims := tokenClaims{
		Purpose: purpose, FamilyID: familyID, AuthVersion: authVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: m.config.Issuer, Subject: userID, Audience: jwt.ClaimStrings{audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt), IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(now), ID: jti,
		},
	}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign %s token: %w", purpose, err)
	}
	return raw, expiresAt, nil
}

func (m *TokenManager) parseAccess(raw string) (tokenClaims, error) {
	return m.parse(raw, accessPurpose, m.config.AccessAudience, m.config.AccessSecret)
}

func (m *TokenManager) parseRefresh(raw string) (tokenClaims, error) {
	return m.parse(raw, refreshPurpose, m.config.RefreshAudience, m.config.RefreshSecret)
}

func (m *TokenManager) parse(raw, purpose, audience, secret string) (tokenClaims, error) {
	claims := tokenClaims{}
	token, err := jwt.ParseWithClaims(raw, &claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errInvalidToken
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(m.config.Issuer), jwt.WithAudience(audience), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithTimeFunc(m.now))
	if err != nil || !token.Valid || claims.Purpose != purpose || claims.Subject == "" || claims.ID == "" || claims.FamilyID == "" || claims.AuthVersion < 1 {
		return tokenClaims{}, errInvalidToken
	}
	return claims, nil
}

func tokenDigest(raw string) []byte {
	digest := sha256.Sum256([]byte(raw))
	return digest[:]
}
