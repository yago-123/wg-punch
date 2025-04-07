package server

import (
	"context"
	"net/http"
	"time"

	"github.com/yago-123/wg-punch/pkg/rendez/store"

	"github.com/gin-gonic/gin"
)

const (
	ServerReadTimeout  = 5 * time.Second
	ServerWriteTimeout = 5 * time.Second
	ServerIdleTimeout  = 10 * time.Second
	MaxHeaderBytes     = 1 << 20
)

type RendezvousServer struct {
	handlers   *Handler
	httpServer *http.Server
}

func NewRendezvous(s store.Store) *RendezvousServer {
	return &RendezvousServer{
		handlers: NewHandler(s),
	}
}

func (s *RendezvousServer) Start(addr string) error {
	r := gin.Default()

	// todo(): add API versioning
	r.POST("/register", s.handlers.RegisterHandler)
	r.GET("/peer/:peer_id", s.handlers.LookupHandler)

	s.httpServer = &http.Server{
		Addr:           addr,
		Handler:        r,
		ReadTimeout:    ServerReadTimeout,
		WriteTimeout:   ServerWriteTimeout,
		IdleTimeout:    ServerIdleTimeout,
		MaxHeaderBytes: MaxHeaderBytes,
	}

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

	return nil
}

func (s *RendezvousServer) Stop(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}
