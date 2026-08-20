package dispatch

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
	"time"
)

// RetryClass categorizes errors for dispatch retry policy.
type RetryClass int

const (
	RetryNone RetryClass = iota
	RetryTransient
	RetryTimeout
)

// ClassifyError maps network and HTTP failures to retry classes.
func ClassifyError(err error, statusCode int) RetryClass {
	if err == nil {
		if statusCode >= 500 {
			return RetryTransient
		}
		return RetryNone
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return RetryTimeout
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return RetryTimeout
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "connection refused"),
		strings.Contains(msg, "no such host"),
		strings.Contains(msg, "timeout"):
		return RetryTransient
	default:
		return RetryNone
	}
}

// ShouldRetry reports whether another attempt is warranted.
func ShouldRetry(class RetryClass, attempt, maxAttempts int) bool {
	if attempt >= maxAttempts {
		return false
	}
	return class == RetryTransient || class == RetryTimeout
}

// Backoff returns exponential backoff duration for an attempt index.
func Backoff(base time.Duration, attempt int) time.Duration {
	if base <= 0 {
		base = 100 * time.Millisecond
	}
	if attempt < 1 {
		attempt = 1
	}
	shift := attempt - 1
	if shift > 10 {
		shift = 10
	}
	return base * time.Duration(1<<shift)
}

// ValidateEndpoint ensures the PLC endpoint is an absolute HTTP URL.
func ValidateEndpoint(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("PLC endpoint must use http or https")
	}
	if strings.TrimSpace(u.Host) == "" {
		return errors.New("PLC endpoint host is required")
	}
	return nil
}
