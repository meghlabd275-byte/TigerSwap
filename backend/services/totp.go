package services

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// RFC 6238 TOTP over RFC 4226 HOTP. The parameters are the ones every
// authenticator app assumes by default (SHA-1, 6 digits, 30-second step), so a
// secret enrolled here works in Google Authenticator, Authy, 1Password and the
// rest without the user configuring anything.
const (
	totpDigits = 6
	totpPeriod = 30 * time.Second
	// totpSkewSteps is how many steps before and after "now" are accepted. One
	// step of tolerance covers ordinary clock drift without widening the window
	// enough to make a code guessable.
	totpSkewSteps = 1
)

// generateTOTPSecret returns a fresh 160-bit secret, base32-encoded without
// padding, which is the format authenticator apps expect.
func generateTOTPSecret() (string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw), nil
}

// decodeTOTPSecret normalises a base32 secret for HMAC use. It accepts
// lower-case input and missing padding so a user can paste a secret as their
// app displays it.
func decodeTOTPSecret(secret string) ([]byte, error) {
	normalised := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(secret), " ", ""))
	normalised = strings.TrimRight(normalised, "=")
	return base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(normalised)
}

// hotp computes the RFC 4226 value for a counter.
func hotp(key []byte, counter uint64, digits int) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(buf[:])
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	truncated := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff

	mod := uint32(1)
	for i := 0; i < digits; i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", digits, truncated%mod)
}

// totpCodeAt returns the code for a secret at an instant.
func totpCodeAt(secret string, at time.Time) (string, error) {
	key, err := decodeTOTPSecret(secret)
	if err != nil {
		return "", err
	}
	return hotp(key, uint64(at.Unix()/int64(totpPeriod.Seconds())), totpDigits), nil
}

// verifyTOTP reports whether code is valid for the secret within the skew
// window. Comparison is constant-time so a wrong code cannot be narrowed down by
// timing.
func verifyTOTP(secret, code string) bool {
	code = strings.TrimSpace(code)
	if len(code) != totpDigits {
		return false
	}
	key, err := decodeTOTPSecret(secret)
	if err != nil {
		return false
	}

	now := time.Now()
	counter := uint64(now.Unix() / int64(totpPeriod.Seconds()))
	for delta := -totpSkewSteps; delta <= totpSkewSteps; delta++ {
		candidate := counter + uint64(delta)
		if subtle.ConstantTimeCompare([]byte(hotp(key, candidate, totpDigits)), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

// totpProvisioningURI builds the otpauth:// URI an authenticator app scans. The
// account name is the user's email so multiple accounts are distinguishable.
func totpProvisioningURI(issuer, account, secret string) string {
	label := url.PathEscape(issuer + ":" + account)
	params := url.Values{}
	params.Set("secret", secret)
	params.Set("issuer", issuer)
	params.Set("algorithm", "SHA1")
	params.Set("digits", fmt.Sprintf("%d", totpDigits))
	params.Set("period", fmt.Sprintf("%d", int(totpPeriod.Seconds())))
	return "otpauth://totp/" + label + "?" + params.Encode()
}
