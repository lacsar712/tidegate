package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime thresholds, secrets, and integration endpoints.
type Config struct {
	ListenAddr         string
	HMACSecret         string
	BypassToken        string
	BypassEnabled      bool
	HeadDiffLimitCM    int
	SmoothN            int
	SkewSec            int
	TicketTTL          time.Duration
	PLCEndpoint        string
	PLCTimeout         time.Duration
	PLCRetries         int
	CircuitThreshold   int
	CircuitCooldown    time.Duration
	JournalPath        string
	DefaultChamberID   string
	RateLimitPerMinute int
}

// Default returns production-safe defaults for a single-chamber demo deployment.
func Default() Config {
	secret := envOr("TIDEGATE_HMAC_SECRET", "dev-hmac-secret-change-me")
	return Config{
		ListenAddr:         envOr("TIDEGATE_LISTEN", ":8080"),
		HMACSecret:         secret,
		BypassToken:        envOr("TIDEGATE_BYPASS_TOKEN", "test-bypass-only"),
		BypassEnabled:      envBool("TIDEGATE_BYPASS_ENABLED", false),
		HeadDiffLimitCM:    envInt("TIDEGATE_HEAD_DIFF_LIMIT_CM", 30),
		SmoothN:            envInt("TIDEGATE_SMOOTH_N", 5),
		SkewSec:            envInt("TIDEGATE_SKEW_SEC", 120),
		TicketTTL:          envDuration("TIDEGATE_TICKET_TTL", 15*time.Second),
		PLCEndpoint:        envOr("TIDEGATE_PLC_ENDPOINT", "http://127.0.0.1:9090/plc/permit"),
		PLCTimeout:         envDuration("TIDEGATE_PLC_TIMEOUT", 2*time.Second),
		PLCRetries:         envInt("TIDEGATE_PLC_RETRIES", 2),
		CircuitThreshold:   envInt("TIDEGATE_CIRCUIT_THRESHOLD", 5),
		CircuitCooldown:    envDuration("TIDEGATE_CIRCUIT_COOLDOWN", 30*time.Second),
		JournalPath:        envOr("TIDEGATE_JOURNAL_PATH", "tidegate.journal"),
		DefaultChamberID:   envOr("TIDEGATE_DEFAULT_CHAMBER", "chamber-1"),
		RateLimitPerMinute: envInt("TIDEGATE_RATE_LIMIT", 120),
	}
}

// Validate ensures mandatory configuration is present and coherent.
func (c Config) Validate() error {
	if strings.TrimSpace(c.ListenAddr) == "" {
		return fmt.Errorf("listen address is required")
	}
	if strings.TrimSpace(c.HMACSecret) == "" {
		return fmt.Errorf("HMAC secret must not be empty")
	}
	if c.HeadDiffLimitCM < 0 {
		return fmt.Errorf("head diff limit must be non-negative")
	}
	if c.SmoothN < 1 {
		return fmt.Errorf("smooth window must be at least 1")
	}
	if c.SkewSec < 1 {
		return fmt.Errorf("skew window must be at least 1 second")
	}
	if c.TicketTTL <= 0 {
		return fmt.Errorf("ticket TTL must be positive")
	}
	if c.PLCRetries < 0 {
		return fmt.Errorf("PLC retries must be non-negative")
	}
	if c.CircuitThreshold < 1 {
		return fmt.Errorf("circuit threshold must be at least 1")
	}
	if c.RateLimitPerMinute < 1 {
		return fmt.Errorf("rate limit must be at least 1")
	}
	return nil
}

// NewRandomSecret generates a hex-encoded random secret for bootstrap scripts.
func NewRandomSecret(nBytes int) (string, error) {
	if nBytes < 8 {
		return "", fmt.Errorf("secret length must be at least 8 bytes")
	}
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

func envBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if raw == "" {
		return fallback
	}
	switch raw {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func envDuration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return d
}
