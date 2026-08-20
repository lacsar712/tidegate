package ingest_test

import (
	"testing"
	"time"

	"github.com/lacsar712/tidegate/internal/ingest"
)

func TestHMACVerify(t *testing.T) {
	secret := "test-secret"
	v, err := ingest.NewVerifier(secret, 60)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"chamberId":"c1"}`)
	now := time.Now().UTC()
	key, ts := v.Sign(body, now)
	headers := map[string]string{"X-Tide-Key": key, "X-Tide-Time": ts}
	if err := v.Verify(headers, body, now); err != nil {
		t.Fatalf("verify failed: %v", err)
	}
}

func TestHMACRejectBadKey(t *testing.T) {
	v, _ := ingest.NewVerifier("secret", 60)
	body := []byte("{}")
	now := time.Now().UTC()
	headers := map[string]string{"X-Tide-Key": "bad", "X-Tide-Time": "123"}
	if err := v.Verify(headers, body, now); err == nil {
		t.Fatal("expected verification failure")
	}
}

func TestEmptySecretRejected(t *testing.T) {
	if _, err := ingest.NewVerifier("", 60); err == nil {
		t.Fatal("empty secret must fail")
	}
}

func TestRateLimiter(t *testing.T) {
	l := ingest.NewRateLimiter(2)
	now := time.Now().UTC()
	if !l.Allow(now) || !l.Allow(now) || l.Allow(now) {
		t.Fatal("expected limit of 2 per minute")
	}
}
