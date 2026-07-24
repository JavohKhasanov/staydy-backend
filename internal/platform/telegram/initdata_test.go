package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"testing"
	"time"
)

// signInitData builds a valid signed initData string for a given token+user (independent of the
// production helper, to cross-check the signing scheme).
func signInitData(token string, userID int64, authDate int64) string {
	user := fmt.Sprintf(`{"id":%d,"first_name":"T"}`, userID)
	fields := map[string]string{
		"user":      user,
		"auth_date": fmt.Sprintf("%d", authDate),
		"query_id":  "AAA",
	}
	// data-check-string: sorted key=value joined by \n (auth_date, query_id, user)
	dcs := "auth_date=" + fields["auth_date"] + "\nquery_id=" + fields["query_id"] + "\nuser=" + fields["user"]
	secret := hmacSHA256([]byte("WebAppData"), []byte(token))
	h := hex.EncodeToString(hmacSHA256(secret, []byte(dcs)))
	v := url.Values{}
	for k, val := range fields {
		v.Set(k, val)
	}
	v.Set("hash", h)
	return v.Encode()
}

func TestVerifyInitData_ValidAndTampered(t *testing.T) {
	const token = "123456:test-bot-token"
	data := signInitData(token, 987654321, time.Now().Unix())

	id, err := VerifyInitData(data, token)
	if err != nil || id != 987654321 {
		t.Fatalf("valid initData: got id=%d err=%v, want 987654321/nil", id, err)
	}

	if _, err := VerifyInitData(data, "wrong-token"); err == nil {
		t.Fatal("wrong token should fail")
	}
	if _, err := VerifyInitData(data+"x", token); err == nil {
		t.Fatal("tampered data should fail")
	}
}

func TestVerifyInitData_Stale(t *testing.T) {
	const token = "123456:test-bot-token"
	old := time.Now().Add(-48 * time.Hour).Unix()
	if _, err := VerifyInitData(signInitData(token, 1, old), token); err == nil {
		t.Fatal("stale initData should fail")
	}
}

func TestVerifyInitData_Garbage(t *testing.T) {
	if _, err := VerifyInitData("nothash=1", "tok"); err == nil {
		t.Fatal("missing hash should fail")
	}
}

// sanity: our helper matches crypto/hmac directly
func TestHmacHelper(t *testing.T) {
	m := hmac.New(sha256.New, []byte("k"))
	m.Write([]byte("d"))
	if hex.EncodeToString(hmacSHA256([]byte("k"), []byte("d"))) != hex.EncodeToString(m.Sum(nil)) {
		t.Fatal("hmac helper mismatch")
	}
}
