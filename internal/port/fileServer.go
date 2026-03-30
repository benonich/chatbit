package port

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
)

const StaticPath = "./dist/"

type fileHandler struct{}

func (s *Server) StartFileServer() {
	servFile := &fileHandler{}

	s.servMux.Handle("/", servFile)
}

func (h *fileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(StaticPath, r.URL.Path)

	slog.Debug("file server request", slog.String("path", path))

	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		http.ServeFile(w, r, filepath.Join(StaticPath, "index.html"))

		slog.Debug("file is not exist and file server will provide index.html", slog.String("path", path))

		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		slog.Error("can't read file from file server", slog.String("path", path), slog.String("error", err.Error()))
		return
	}

	http.FileServer(http.Dir(StaticPath)).ServeHTTP(w, r)
}
