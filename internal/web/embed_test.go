package web_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lacsar712/tidegate/internal/web"
)

func TestStaticIndex(t *testing.T) {
	mux := http.NewServeMux()
	web.Mount(mux)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if len(body) == 0 {
		t.Fatal("expected index content")
	}
}

func TestStaticFS(t *testing.T) {
	fs := web.StaticFS()
	f, err := fs.Open("index.html")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
}
