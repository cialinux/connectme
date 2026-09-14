package identity

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func NewTOTP(email string) (secret, uri string, err error) {
	b := make([]byte, 20)
	if _, err = rand.Read(b); err != nil {
		return
	}
	secret = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
	v := url.Values{"secret": {secret}, "issuer": {"ConnectMe"}, "algorithm": {"SHA1"}, "digits": {"6"}, "period": {"30"}}
	uri = "otpauth://totp/ConnectMe:" + url.PathEscape(email) + "?" + v.Encode()
	return
}
func VerifyTOTP(secret, code string, now time.Time) bool {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil {
		return false
	}
	for offset := -1; offset <= 1; offset++ {
		counter := uint64(now.Unix()/30 + int64(offset))
		var msg [8]byte
		for i := 7; i >= 0; i-- {
			msg[i] = byte(counter)
			counter >>= 8
		}
		mac := hmac.New(sha1.New, key)
		_, _ = mac.Write(msg[:])
		sum := mac.Sum(nil)
		o := sum[len(sum)-1] & 15
		n := (uint32(sum[o])&127)<<24 | uint32(sum[o+1])<<16 | uint32(sum[o+2])<<8 | uint32(sum[o+3])
		expected := fmt.Sprintf("%06d", n%1000000)
		if hmac.Equal([]byte(expected), []byte(code)) {
			return true
		}
	}
	return false
}
func recoveryCode() string {
	b := make([]byte, 10)
	_, _ = rand.Read(b)
	s := strings.ToUpper(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b))
	return s[:4] + "-" + s[4:8] + "-" + s[8:12] + "-" + s[12:16]
}
func normalizeCode(s string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(s), "-", ""))
}
func parseInt(s string) int { n, _ := strconv.Atoi(s); return n }
