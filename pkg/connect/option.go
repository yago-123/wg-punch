package connect

import (
	"time"

	"github.com/go-logr/logr"
)

const (
	defaultRendezServer = "http://rendezvous.yago.ninja:7777"
	defaultWaitInterval = 1 * time.Second
)

type config struct {
	rendezServerURL string
	waitInterval    time.Duration
	stunServers     []string
	logger          logr.Logger
}

func newDefaultConfig() *config {
	return &config{
		rendezServerURL: defaultRendezServer,
		waitInterval:    defaultWaitInterval,
		logger:          logr.Discard(),
	}
}

type Option func(*config)

// WithRendezServer sets the rendezvous server URL. The server must implement interface
func WithRendezServer(server string) Option {
	return func(cfg *config) {
		cfg.rendezServerURL = server
	}
}

// WithWaitInterval sets the wait interval for the connector. The interval must be greater than 0
func WithWaitInterval(interval time.Duration) Option {
	return func(cfg *config) {
		cfg.waitInterval = interval
	}
}

// WithLogger sets the logger to use for logging. The logger must implement the logr.Logger interface
func WithLogger(logger logr.Logger) Option {
	return func(cfg *config) {
		cfg.logger = logger
	}
}
