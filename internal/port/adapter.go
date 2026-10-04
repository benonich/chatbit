package port

import (
	"chatbit/internal/core"
	"chatbit/internal/domain"
	"errors"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 10 * time.Second
	idleTimeout       = 120 * time.Second
)

type BuildInfo struct {
	Version   string
	Commit    string
	BuildDate string
}

type Server struct {
	app       *core.Application
	Server    *http.Server
	servMux   *http.ServeMux
	buildInfo BuildInfo
}

func NewServer(app *core.Application, info BuildInfo) *Server {
	return &Server{app: app, buildInfo: info}
}

func (s *Server) Start() error {
	// WebSocket connection
	s.servMux = http.NewServeMux()

	s.StartFileServer(s.buildInfo)

	s.StartWebSocketServer()

	s.startFileCleanup()

	servObj := &http.Server{
		Addr:              ":" + "8081",
		Handler:           s.servMux,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	if err := servObj.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}

	return nil
}

func (s *Server) startFileCleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for range ticker.C {
			slog.Info("file cleanup started")

			err := filepath.WalkDir(domain.FileDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				fileID, err := uuid.Parse(d.Name())
				if err != nil {
					return nil
				}
				_, err = s.app.DB.File.Get(fileID[:])
				if err != nil {
					os.Remove(path)
					os.Remove(filepath.Dir(path))
				}
				return nil
			})
			if err != nil {
				slog.Error("file cleanup failed", slog.String("error", err.Error()))
			}
		}
	}()
}
