package server

import (
	"context"
	"net/http"

	"wg-punch/pkg/rendezvous/store"

	"github.com/gin-gonic/gin"
)

type RendezvousServer struct {
	handlers   *Handler
	httpServer *http.Server
}

func NewRendezvousServer(s store.Store) *RendezvousServer {
	return &RendezvousServer{
		handlers: NewHandler(s),
	}
}

func (s *RendezvousServer) Start(addr string) error {
	r := gin.Default()

	r.POST("/register", s.handlers.RegisterHandler)
	r.GET("/peer/:peer_id", s.handlers.LookupHandler)

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: r,
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
