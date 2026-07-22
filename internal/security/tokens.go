package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
)

// AccessClaims is the JWT payload for short-lived access tokens.
type AccessClaims struct {
	OrgID    string `json:"org,omitempty"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// TokenManager issues and verifies access tokens (HS256 JWT) and refresh tokens
// (opaque random strings; the server stores only their hash).
type TokenManager struct {
	issuer     string
	signingKey []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewTokenManager(issuer, signingKey string, accessTTL, refreshTTL time.Duration) *TokenManager {
	return &TokenManager{
		issuer:     issuer,
		signingKey: []byte(signingKey),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

func (m *TokenManager) AccessTTL() time.Duration  { return m.accessTTL }
func (m *TokenManager) RefreshTTL() time.Duration { return m.refreshTTL }

// GenerateAccessToken signs a JWT for the principal and returns it with its expiry.
func (m *TokenManager) GenerateAccessToken(p entity.Principal) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(m.accessTTL)
	orgID := ""
	if p.OrgID != uuid.Nil {
		orgID = p.OrgID.String()
	}
	claims := AccessClaims{
		OrgID:    orgID,
		Email:    p.Email,
		FullName: p.FullName,
		Role:     string(p.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   p.UserID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.signingKey)
	return signed, expiresAt, err
}

// StudentTokenTTL is how long a student mini-app session lasts. Students get a single long-lived
// access token (no refresh rotation) — the friction of frequent re-login isn't worth it for them,
// and the token carries no back-office authority.
const StudentTokenTTL = 30 * 24 * time.Hour

// GenerateStudentToken signs a long-lived access token for a student. Subject is the student's id,
// Role is "student"; there is no refresh token.
func (m *TokenManager) GenerateStudentToken(p entity.Principal) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(StudentTokenTTL)
	claims := AccessClaims{
		OrgID:    p.OrgID.String(),
		FullName: p.FullName,
		Role:     string(entity.RoleStudent),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   p.UserID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.signingKey)
	return signed, expiresAt, err
}

// ParseAccessToken verifies a token and reconstructs the Principal from its claims.
func (m *TokenManager) ParseAccessToken(token string) (entity.Principal, error) {
	var claims AccessClaims
	parsed, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		return m.signingKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !parsed.Valid {
		return entity.Principal{}, errors.New("invalid access token")
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return entity.Principal{}, errors.New("invalid subject")
	}
	var orgID uuid.UUID
	if claims.OrgID != "" {
		orgID, err = uuid.Parse(claims.OrgID)
		if err != nil {
			return entity.Principal{}, errors.New("invalid org claim")
		}
	}
	return entity.Principal{
		UserID:   userID,
		OrgID:    orgID,
		Email:    claims.Email,
		FullName: claims.FullName,
		Role:     entity.UserRole(claims.Role),
	}, nil
}

// GenerateRefreshToken returns a new opaque refresh token (256 bits of randomness, hex).
func (m *TokenManager) GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// HashRefreshToken returns the SHA-256 hex digest stored server-side for a refresh token.
func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
