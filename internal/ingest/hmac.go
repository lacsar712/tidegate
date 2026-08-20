package ingest

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	headerKey  = "X-Tide-Key"
	headerTime = "X-Tide-Time"
)

// Verifier validates HMAC-authenticated sensor payloads.
type Verifier struct {
	secret  string
	skewSec int
}

// NewVerifier constructs an HMAC verifier with allowed clock skew.
func NewVerifier(secret string, skewSec int) (*Verifier, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, fmt.Errorf("HMAC secret must not be empty")
	}
	if skewSec < 1 {
		return nil, fmt.Errorf("skew window must be positive")
	}
	return &Verifier{secret: secret, skewSec: skewSec}, nil
}

// Verify checks headers and body integrity.
func (v *Verifier) Verify(headers map[string]string, body []byte, now time.Time) error {
	key := strings.TrimSpace(headers[headerKey])
	if key == "" {
		return fmt.Errorf("missing %s header", headerKey)
	}
	tsRaw := strings.TrimSpace(headers[headerTime])
	if tsRaw == "" {
		return fmt.Errorf("missing %s header", headerTime)
	}
	tsInt, err := strconv.ParseInt(tsRaw, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid %s header", headerTime)
	}
	ts := time.Unix(tsInt, 0).UTC()
	skew := now.Sub(ts)
	if skew < 0 {
		skew = -skew
	}
	if int(skew.Seconds()) > v.skewSec {
		return fmt.Errorf("request timestamp outside allowed skew")
	}

	expected := v.sign(tsRaw, body)
	if !hmac.Equal([]byte(key), []byte(expected)) {
		return fmt.Errorf("HMAC verification failed")
	}
	return nil
}

// Sign produces the header value clients must send.
func (v *Verifier) Sign(body []byte, now time.Time) (keyHeader, timeHeader string) {
	ts := strconv.FormatInt(now.Unix(), 10)
	return v.sign(ts, body), ts
}

func (v *Verifier) sign(ts string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(v.secret))
	_, _ = mac.Write([]byte(ts))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// HeaderNames returns canonical header names for documentation.
func HeaderNames() (key, time string) {
	return headerKey, headerTime
}
