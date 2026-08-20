package ingest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

// SignedClient posts authenticated sensor payloads.
type SignedClient struct {
	baseURL  string
	verifier *Verifier
	client   *http.Client
}

// NewSignedClient creates an ingest client for integrators and tests.
func NewSignedClient(baseURL string, verifier *Verifier) *SignedClient {
	return &SignedClient{
		baseURL:  baseURL,
		verifier: verifier,
		client:   &http.Client{Timeout: 5 * time.Second},
	}
}

// PostJSON signs and sends a JSON payload to path.
func (c *SignedClient) PostJSON(path string, payload any, now time.Time) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	key, ts := c.verifier.Sign(body, now)
	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerKey, key)
	req.Header.Set(headerTime, ts)
	return c.client.Do(req)
}
