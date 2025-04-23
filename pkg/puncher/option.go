package puncher

import (
	"time"

	"github.com/go-logr/logr"
)

const (
	defaultPuncherInterval = 300 * time.Millisecond
)

var (
	defaultSTUNServers = []string{
		"stun.l.google.com:19302",
		"stun1.l.google.com:19302",
	}
)

type config struct {
	puncherInterval time.Duration
	stunServers     []string
	logger          logr.Logger
}

type Option func(*config)

func newDefaultConfig() *config {
	return &config{
		puncherInterval: defaultPuncherInterval,
		stunServers:     defaultSTUNServers,
		logger:          logr.Discard(),
	}
}

// WithPuncherInterval sets the interval for sending UDP packets. The interval must be greater than 0
func WithPuncherInterval(interval time.Duration) Option {
	return func(cfg *config) {
		cfg.puncherInterval = interval
	}
}

// WithSTUNServers sets the STUN servers to use for hole punching. The servers must be reachable
func WithSTUNServers(servers []string) Option {
	return func(cfg *config) {
		cfg.stunServers = servers
	}
}

// WithLogger sets the logger to use for logging. The logger must implement the logr.Logger interface
func WithLogger(logger logr.Logger) Option {
	return func(cfg *config) {
		cfg.logger = logger
	}
}
