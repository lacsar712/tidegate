package app

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/lacsar712/tidegate/internal/permit"
)

func TestPermitHTTPStatusUsesErrorsIs(t *testing.T) {
	wrapped := fmt.Errorf("%w", permit.ErrTicketExpired)
	if status := permitHTTPStatus(wrapped); status != http.StatusGone {
		t.Fatalf("wrapped ErrTicketExpired must map to 410 Gone via errors.Is; got %d", status)
	}
	// Equality against the sentinel fails once wrapped — errors.Is is required.
	if wrapped == permit.ErrTicketExpired {
		t.Fatal("sanity: wrapped error should not be == sentinel")
	}
	if !errors.Is(wrapped, permit.ErrTicketExpired) {
		t.Fatal("sanity: errors.Is must succeed on %w wrap")
	}
}
