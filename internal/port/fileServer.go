package port

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
)

const StaticPath = "./dist/"

func (s *Server) StartFileServer() {
	servFile, err := NewFileHandler(os.Getenv("VAPID_PUBLIC_KEY"))
	if err != nil {
		slog.Error("failed to create file handler", slog.String("error", err.Error()))
		return
	}

	s.servMux.Handle("/", servFile)
}

type fileHandler struct {
	static      http.Handler
	tmpl        *template.Template
	vapidPubKey string
}

type PageData struct {
	VAPIDPublicKey string
}

func NewFileHandler(vapidPubKey string) (*fileHandler, error) {
	tmpl, err := template.ParseFiles(filepath.Join(StaticPath, "index.html"))
	if err != nil {
		return nil, fmt.Errorf("parse index.html: %w", err)
	}

	return &fileHandler{
		static:      http.FileServer(http.Dir(StaticPath)),
		tmpl:        tmpl,
		vapidPubKey: vapidPubKey,
	}, nil
}

func (h *fileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(StaticPath, filepath.Clean(r.URL.Path))

	slog.Debug("file server request", slog.String("path", path))

	_, err := os.Stat(path)
	switch {
	case os.IsNotExist(err):
		slog.Debug("file not found, serving index.html", slog.String("path", path))
		h.serveIndex(w, r)

	case err != nil:
		slog.Error("file server error", slog.String("path", path), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)

	default:
		h.static.ServeHTTP(w, r)
	}
}

func (h *fileHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.Execute(w, PageData{
		VAPIDPublicKey: h.vapidPubKey,
	}); err != nil {
		slog.Error("template render error", slog.String("error", err.Error()))
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
