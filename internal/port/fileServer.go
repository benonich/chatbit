package port

import (
	"bytes"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
)

const StaticPath = "./dist/"

func (s *Server) StartFileServer(info BuildInfo) {
	servFile, err := NewFileHandler(os.Getenv("VAPID_PUBLIC_KEY"), info)
	if err != nil {
		slog.Error("failed to create file handler", slog.String("error", err.Error()))
		return
	}

	s.servMux.Handle("/", servFile)
}

type fileHandler struct {
	static    http.Handler
	indexHTML []byte
	swJS      []byte
}

type templateData struct {
	VAPIDPublicKey string
	Version        string
	Commit         string
	BuildDate      string
}

func NewFileHandler(vapidPubKey string, info BuildInfo) (*fileHandler, error) {
	data := templateData{
		VAPIDPublicKey: vapidPubKey,
		Version:        info.Version,
		Commit:         info.Commit,
		BuildDate:      info.BuildDate,
	}

	indexHTML, err := renderTemplate(filepath.Join(StaticPath, "index.html"), data)
	if err != nil {
		return nil, fmt.Errorf("render index.html: %w", err)
	}

	swJS, err := renderTemplate(filepath.Join(StaticPath, "service-worker.js"), data)
	if err != nil {
		return nil, fmt.Errorf("render service-worker.js: %w", err)
	}

	return &fileHandler{
		static:    http.FileServer(http.Dir(StaticPath)),
		indexHTML: indexHTML,
		swJS:      swJS,
	}, nil
}

func renderTemplate(path string, data any) ([]byte, error) {
	tmpl, err := template.ParseFiles(path)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (h *fileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Intercept service worker — serve rendered version with injected build info
	if r.URL.Path == "/service-worker.js" {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(h.swJS)
		return
	}

	path := filepath.Join(StaticPath, filepath.Clean(r.URL.Path))

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

func (h *fileHandler) serveIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(h.indexHTML)
}
