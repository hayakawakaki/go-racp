package server

import (
	"fmt"

	"github.com/hayakawakaki/go-racp/server/config"
)

type Server struct {
	Cfg *config.Config
}

func NewServer() (*Server, error) {
	cfg, err := config.NewConfig()
	if err != nil {
		return nil, fmt.Errorf("server: failed to load config: %w", err)
	}

	return &Server{
		Cfg: cfg,
	}, nil
}

func (s *Server) Run() {
	fmt.Println("App is running")
}
