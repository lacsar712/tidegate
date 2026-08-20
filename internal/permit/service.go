package permit

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/lacsar712/tidegate/internal/interlock"
	"github.com/lacsar712/tidegate/internal/level"
	"github.com/lacsar712/tidegate/internal/model"
)

// Service issues and validates permit tickets.
type Service struct {
	mu            sync.Mutex
	ttl           time.Duration
	headLimit     int
	bypassEnabled bool
	bypassToken   string
	nonces        map[string]struct{}
	ledger        *NonceLedger
	tickets       map[string]model.PermitTicket
	levels        *level.Store
	interlock     *interlock.Evaluator
}

// NewService constructs a permit issuer bound to level and interlock modules.
func NewService(levels *level.Store, evaluator *interlock.Evaluator, headLimit int, ttl time.Duration, bypassEnabled bool, bypassToken string) *Service {
	return &Service{
		ttl:           ttl,
		headLimit:     headLimit,
		bypassEnabled: bypassEnabled,
		bypassToken:   bypassToken,
		nonces:        make(map[string]struct{}),
		ledger:        NewNonceLedger(ttl),
		tickets:       make(map[string]model.PermitTicket),
		levels:        levels,
		interlock:     evaluator,
	}
}

// EvaluateOpenRequest performs head-diff, interlock, and ticket issuance.
func (s *Service) EvaluateOpenRequest(req model.OpenRequest, gates []model.Gate, now time.Time) model.PermitResult {
	if err := req.Validate(); err != nil {
		return model.PermitResult{Allowed: false, Deny: deny("INVALID_REQUEST", err.Error(), req.ChamberID, req.GateID, now)}
	}

	up, down, head, ok := s.levels.Snapshot(req.ChamberID)
	if !ok {
		return model.PermitResult{Allowed: false, Deny: deny("LEVEL_UNAVAILABLE", "smoothed levels not ready", req.ChamberID, req.GateID, now)}
	}
	if level.ExceedsLimit(head, s.headLimit) {
		msg := fmt.Sprintf("head diff %dcm exceeds limit %dcm (up=%d down=%d)", head, s.headLimit, up, down)
		return model.PermitResult{Allowed: false, Deny: deny("HEAD_DIFF", msg, req.ChamberID, req.GateID, now)}
	}

	bypass := interlock.ValidateBypassToken(s.bypassEnabled, s.bypassToken, req.BypassToken)
	allowed, conflict, err := s.interlock.EvaluateRequest(req.GateID, gates, bypass)
	if err != nil {
		return model.PermitResult{Allowed: false, Deny: deny("INTERLOCK_ERROR", err.Error(), req.ChamberID, req.GateID, now)}
	}
	if !allowed {
		msg := interlock.DenyMessage(req.GateID, conflict)
		code := "INTERLOCK"
		if bypass {
			code = "INTERLOCK_BYPASS_DENIED"
		}
		return model.PermitResult{Allowed: false, Deny: deny(code, msg, req.ChamberID, req.GateID, now)}
	}

	ticket, err := s.issue(req, now)
	if err != nil {
		return model.PermitResult{Allowed: false, Deny: deny("ISSUE_FAILED", err.Error(), req.ChamberID, req.GateID, now)}
	}
	return model.PermitResult{Allowed: true, Ticket: &ticket}
}

func (s *Service) issue(req model.OpenRequest, now time.Time) (model.PermitTicket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	nonce, err := randomNonce()
	if err != nil {
		return model.PermitTicket{}, err
	}
	if _, used := s.nonces[nonce]; used {
		return model.PermitTicket{}, fmt.Errorf("nonce collision")
	}
	id := fmt.Sprintf("%s-%s-%d", req.GateID, req.Action, now.UnixNano())
	ticket := model.PermitTicket{
		ID:        id,
		ChamberID: req.ChamberID,
		GateID:    req.GateID,
		Action:    req.Action,
		Nonce:     nonce,
		IssuedAt:  now,
		ExpireAt:  now.Add(s.ttl),
	}
	s.tickets[id] = ticket
	return ticket, nil
}

// ValidateTicket re-checks expiry, nonce, head diff, and interlock before use.
func (s *Service) ValidateTicket(ticketID string, gates []model.Gate, now time.Time) (model.PermitTicket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ticket, ok := s.tickets[ticketID]
	if !ok {
		return model.PermitTicket{}, fmt.Errorf("ticket not found")
	}
	if ticket.Used {
		return model.PermitTicket{}, fmt.Errorf("ticket already used")
	}
	if ticket.Expired(now) {
		return model.PermitTicket{}, fmt.Errorf("%w", ErrTicketExpired)
	}
	if _, seen := s.nonces[ticket.Nonce]; seen {
		return model.PermitTicket{}, fmt.Errorf("nonce conflict")
	}
	if s.ledger.Seen(ticket.Nonce) {
		return model.PermitTicket{}, fmt.Errorf("%w", ErrNonceReplay)
	}

	_, _, head, ok := s.levels.Snapshot(ticket.ChamberID)
	if !ok {
		return model.PermitTicket{}, fmt.Errorf("levels unavailable")
	}
	if level.ExceedsLimit(head, s.headLimit) {
		return model.PermitTicket{}, fmt.Errorf("head diff no longer within limit")
	}
	allowed, conflict, err := s.interlock.EvaluateRequest(ticket.GateID, gates, false)
	if err != nil {
		return model.PermitTicket{}, err
	}
	if !allowed {
		return model.PermitTicket{}, fmt.Errorf(interlock.DenyMessage(ticket.GateID, conflict))
	}
	return ticket, nil
}

// Consume marks a ticket nonce as spent.
func (s *Service) Consume(ticketID string, now time.Time) (model.PermitTicket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ticket, ok := s.tickets[ticketID]
	if !ok {
		return model.PermitTicket{}, fmt.Errorf("ticket not found")
	}
	if ticket.Used {
		return model.PermitTicket{}, fmt.Errorf("%w", ErrTicketUsed)
	}
	if ticket.Expired(now) {
		return model.PermitTicket{}, fmt.Errorf("%w", ErrTicketExpired)
	}
	if s.ledger.Seen(ticket.Nonce) {
		return model.PermitTicket{}, fmt.Errorf("%w", ErrNonceReplay)
	}
	if err := s.ledger.Spend(ticket.Nonce, now); err != nil {
		return model.PermitTicket{}, fmt.Errorf("%w", ErrNonceReplay)
	}
	ticket.Used = true
	s.tickets[ticketID] = ticket
	return ticket, nil
}

// NonceSeen reports whether a permit nonce was recorded as spent.
func (s *Service) NonceSeen(nonce string) bool {
	return s.ledger.Seen(nonce)
}

// Get returns a ticket by id.
func (s *Service) Get(ticketID string) (model.PermitTicket, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tickets[ticketID]
	return t, ok
}

func deny(code, msg, chamberID, gateID string, at time.Time) *model.DenyReason {
	return &model.DenyReason{Code: code, Message: msg, ChamberID: chamberID, GateID: gateID, At: at}
}

func randomNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
