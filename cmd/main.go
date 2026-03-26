package main

import "github.com/hayakawakaki/go-racp/app"

func main() {
	cfg := app.Config{
		Mode: "development",
	}

	cp := app.NewApp(&cfg)
	cp.Run()
}
