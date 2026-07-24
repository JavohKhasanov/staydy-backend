package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// maxInitDataAge rejects replayed initData older than this (Telegram signs a fresh one per open).
const maxInitDataAge = 24 * time.Hour

var ErrInvalidInitData = errors.New("telegram: invalid init data")

// VerifyInitData validates a Telegram Mini App initData string against the bot token and returns
// the authenticated Telegram user id. It follows Telegram's WebApp signing scheme:
//
//	secret_key = HMAC_SHA256(key="WebAppData", msg=bot_token)
//	hash       = HMAC_SHA256(key=secret_key, msg=data_check_string)
//
// where data_check_string is every field except `hash`, sorted by key, joined as key=value by '\n'.
func VerifyInitData(initData, botToken string) (int64, error) {
	values, err := url.ParseQuery(initData)
	if err != nil {
		return 0, ErrInvalidInitData
	}
	hash := values.Get("hash")
	if hash == "" || botToken == "" {
		return 0, ErrInvalidInitData
	}
	values.Del("hash")

	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(values.Get(k))
	}

	secret := hmacSHA256([]byte("WebAppData"), []byte(botToken))
	computed := hex.EncodeToString(hmacSHA256(secret, []byte(b.String())))
	if !hmac.Equal([]byte(computed), []byte(hash)) {
		return 0, ErrInvalidInitData
	}

	// Reject stale/replayed payloads.
	if ad := values.Get("auth_date"); ad != "" {
		if sec, e := strconv.ParseInt(ad, 10, 64); e == nil {
			if time.Since(time.Unix(sec, 0)) > maxInitDataAge {
				return 0, ErrInvalidInitData
			}
		}
	}

	var user struct {
		ID int64 `json:"id"`
	}
	if e := json.Unmarshal([]byte(values.Get("user")), &user); e != nil || user.ID == 0 {
		return 0, ErrInvalidInitData
	}
	return user.ID, nil
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}
