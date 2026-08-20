package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/lacsar712/tidegate/internal/model"
)

// PLCClient sends permit commands to the interlock PLC over HTTP.
type PLCClient struct {
	mu       sync.Mutex
	endpoint string
	timeout  time.Duration
	retries  int
	client   *http.Client
	breaker  *CircuitBreaker
}

// NewPLCClient constructs a PLC dispatch client with circuit breaking.
func NewPLCClient(endpoint string, timeout time.Duration, retries int, threshold int, cooldown time.Duration) *PLCClient {
	return &PLCClient{
		endpoint: endpoint,
		timeout:  timeout,
		retries:  retries,
		client:   &http.Client{Timeout: timeout},
		breaker:  NewCircuitBreaker(threshold, cooldown),
	}
}

// Command is the PLC-bound permit payload.
type Command struct {
	TicketID  string             `json:"ticketId"`
	ChamberID string             `json:"chamberId"`
	GateID    string             `json:"gateId"`
	Action    model.PermitAction `json:"action"`
	Nonce     string             `json:"nonce"`
	ExpireAt  time.Time          `json:"expireAt"`
}

// Send transmits a permit command with retries and breaker awareness.
func (c *PLCClient) Send(ctx context.Context, ticket model.PermitTicket) (model.DispatchOutcome, error) {
	if c.breaker.Open() {
		return model.DispatchOutcome{}, fmt.Errorf("circuit open")
	}
	cmd := Command{
		TicketID:  ticket.ID,
		ChamberID: ticket.ChamberID,
		GateID:    ticket.GateID,
		Action:    ticket.Action,
		Nonce:     ticket.Nonce,
		ExpireAt:  ticket.ExpireAt,
	}
	body, err := json.Marshal(cmd)
	if err != nil {
		return model.DispatchOutcome{}, err
	}

	var lastErr error
	attempts := c.retries + 1
	for i := 0; i < attempts; i++ {
		if ctx.Err() != nil {
			return model.DispatchOutcome{}, ctx.Err()
		}
		outcome, sendErr := c.doOnce(ctx, body, ticket)
		if sendErr == nil && outcome.Success {
			c.breaker.RecordSuccess()
			return outcome, nil
		}
		lastErr = sendErr
		if outcome.StatusCode >= 500 {
			c.breaker.RecordFailure(time.Now().UTC())
			continue
		}
		if sendErr != nil {
			c.breaker.RecordFailure(time.Now().UTC())
			continue
		}
		return outcome, fmt.Errorf("plc rejected command: %s", outcome.Message)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("plc dispatch failed")
	}
	return model.DispatchOutcome{}, lastErr
}

func (c *PLCClient) doOnce(ctx context.Context, body []byte, ticket model.PermitTicket) (model.DispatchOutcome, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return model.DispatchOutcome{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	attempted := time.Now().UTC()
	outcome := model.DispatchOutcome{
		GateID:    ticket.GateID,
		Action:    string(ticket.Action),
		Attempted: attempted,
	}
	if err != nil {
		outcome.Message = err.Error()
		return outcome, err
	}
	defer resp.Body.Close()
	outcome.StatusCode = resp.StatusCode
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	outcome.Message = string(respBody)
	outcome.Success = resp.StatusCode >= 200 && resp.StatusCode < 300
	return outcome, nil
}

// BreakerState exposes circuit breaker health for status APIs.
func (c *PLCClient) BreakerState() CircuitState {
	return c.breaker.State(time.Now().UTC())
}
