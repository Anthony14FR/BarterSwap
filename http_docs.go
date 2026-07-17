package main

import (
	"embed"
	"io/fs"
	"net/http"
)

// docsFiles embeds the OpenAPI spec and the Swagger UI assets into the
// binary, so /docs works offline without touching the file system.
//
//go:embed docs
var docsFiles embed.FS

func registerDocs(mux *http.ServeMux) {
	sub, err := fs.Sub(docsFiles, "docs")
	if err != nil {
		panic("docs: " + err.Error())
	}
	mux.Handle("GET /docs/", http.StripPrefix("/docs/", http.FileServerFS(sub)))
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/docs/", http.StatusMovedPermanently)
	})
}
