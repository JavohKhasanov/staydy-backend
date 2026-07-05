package entity

import (
	"time"

	"github.com/google/uuid"
)

// RefreshSession is a server-side record of an issued refresh token. Only the token's
// hash is stored; the opaque token itself lives only on the client.
type RefreshSession struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	OrgID     uuid.UUID
	TokenHash string
	UserAgent string
	IP        string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

// Active reports whether the session can still be used to mint a new access token.
func (s RefreshSession) Active(now time.Time) bool {
	return s.RevokedAt == nil && s.ExpiresAt.After(now)
}
