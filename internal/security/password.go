// Package security holds cryptographic operations: JWT access tokens, opaque refresh
// tokens, and password hashing. It depends only on entity.
package security

import "golang.org/x/crypto/bcrypt"

// HashPassword returns a bcrypt hash of the plaintext password.
func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

// CheckPassword reports whether pw matches the bcrypt hash.
func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// dummyHash is a fixed bcrypt hash used to equalize timing on auth paths where the org
// or user is not found, so attackers can't enumerate valid accounts from response latency.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("student-success-timing-equalizer"), bcrypt.DefaultCost)

// DummyCheck runs a throwaway bcrypt comparison so a not-found login path costs roughly
// the same as a real password verification.
func DummyCheck(pw string) { _ = bcrypt.CompareHashAndPassword(dummyHash, []byte(pw)) }
