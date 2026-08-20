package permit

import "errors"

var (
	// ErrNonceReplay indicates a ticket nonce was reused.
	ErrNonceReplay = errors.New("nonce replay detected")
	// ErrTicketExpired indicates the permit TTL elapsed.
	ErrTicketExpired = errors.New("ticket expired")
	// ErrTicketUsed indicates the permit was already consumed.
	ErrTicketUsed = errors.New("ticket already used")
)
