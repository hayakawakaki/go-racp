package app

import "fmt"

type App struct {
	Cfg *Config
}

func NewApp(config *Config) *App {
	return &App{
		Cfg: config,
	}
}

func (a *App) Run() {
	fmt.Println("App is running")
}
