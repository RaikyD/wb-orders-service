package presentation

import (
	"embed"
	"github.com/go-chi/chi/v5"
	"io/fs"
	"net/http"
)

//go:embed web/*
var webFS embed.FS

func MountStatic(r chi.Router) {
	sub, _ := fs.Sub(webFS, "web")

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, sub, "index.html")
	})
	r.Mount("/", http.FileServer(http.FS(sub)))
}
