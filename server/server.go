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
		return nil, err
	}

	return &Server{
		Cfg: cfg,
	}, nil
}

func (s *Server) Run() {
	fmt.Println("App is running")
}
