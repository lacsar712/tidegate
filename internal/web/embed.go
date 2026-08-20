package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var staticFS embed.FS

// Mount registers the operator console and static assets.
func Mount(mux *http.ServeMux) {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))
	mux.Handle("GET /{$}", fileServer)
	mux.Handle("GET /index.html", fileServer)
	mux.Handle("GET /styles.css", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "styles.css"
		fileServer.ServeHTTP(w, r)
	}))
	mux.Handle("GET /app.js", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "app.js"
		fileServer.ServeHTTP(w, r)
	}))
}

// StaticFS exposes embedded files for tests.
func StaticFS() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
