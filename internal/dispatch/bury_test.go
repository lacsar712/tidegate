package dispatch_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lacsar712/tidegate/internal/dispatch"
	"github.com/lacsar712/tidegate/internal/model"
)

func TestSendHonorsCancel(t *testing.T) {
	started := make(chan struct{})
	plc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer plc.Close()

	client := dispatch.NewPLCClient(plc.URL, 5*time.Second, 0, 5, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := client.Send(ctx, model.PermitTicket{
			ID: "t1", ChamberID: "c1", GateID: "g1", Action: model.ActionOpen, Nonce: "n1",
			ExpireAt: time.Now().UTC().Add(time.Minute),
		})
		errCh <- err
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("PLC handler never started")
	}
	cancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected cancel error from Send")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Send did not return promptly after cancel (still blocked on PLC)")
	}
}
