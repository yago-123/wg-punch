package connect

import "github.com/go-logr/logr"

const (
	defaultRendezServer = "http://rendezvous.yago.ninja:7777"
)

var (
	defaultSTUNServers = []string{
		"stun.l.google.com:19302",
		"stun1.l.google.com:19302",
	}
)

type config struct {
	rendezServerURL string
	stunServers     []string
	logger          logr.Logger
}

func newDefaultConfig() *config {
	return &config{
		rendezServerURL: defaultRendezServer,
		stunServers:     defaultSTUNServers,
		logger:          logr.Discard(),
	}
}

type Option func(*config)

func WithRendezServer(server string) Option {
	return func(cfg *config) {
		cfg.rendezServerURL = server
	}
}

func WithSTUNServers(servers []string) Option {
	return func(cfg *config) {
		cfg.stunServers = servers
	}
}

func WithLogger(logger logr.Logger) Option {
	return func(cfg *config) {
		cfg.logger = logger
	}
}
