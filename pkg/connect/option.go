package connect

import "github.com/go-logr/logr"

type Option func(*Connector)

func WithLogger(logger logr.Logger) Option {
	return func(c *Connector) {
		c.logger = logger
	}
}

func WithSTUNServers(servers []string) Option {
	return func(c *Connector) {
		c.stunServers = servers
	}
}
