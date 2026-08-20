package dispatch_test

import (
	"testing"
	"time"

	"github.com/lacsar712/tidegate/internal/dispatch"
)

func TestCircuitBreakerOpensAndClears(t *testing.T) {
	b := dispatch.NewCircuitBreaker(2, time.Second)
	now := time.Now().UTC()
	b.RecordFailure(now)
	if b.Open() {
		t.Fatal("should not open after one failure")
	}
	b.RecordFailure(now)
	if !b.Open() {
		t.Fatal("expected open breaker")
	}
	b.RecordSuccess()
	if b.Open() {
		t.Fatal("success should clear breaker")
	}
}

func TestClassifyError(t *testing.T) {
	if dispatch.ClassifyError(nil, 503) != dispatch.RetryTransient {
		t.Fatal("5xx should be transient")
	}
}

func TestValidateEndpoint(t *testing.T) {
	if err := dispatch.ValidateEndpoint("http://127.0.0.1:9090/plc"); err != nil {
		t.Fatal(err)
	}
	if err := dispatch.ValidateEndpoint("ftp://bad"); err == nil {
		t.Fatal("expected invalid scheme")
	}
}

func TestBackoff(t *testing.T) {
	d := dispatch.Backoff(100*time.Millisecond, 3)
	if d < 300*time.Millisecond {
		t.Fatalf("expected exponential backoff, got %v", d)
	}
}
