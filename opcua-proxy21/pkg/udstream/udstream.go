package udstream

import (
	"opcua-proxy21/pkg/udstream/server"
)

type Config = server.Config
type Handler = server.Handler
type Server = server.Server

func NewServer(config *Config, handler Handler) (*Server, error) {
	return server.NewServer(config, handler)
}