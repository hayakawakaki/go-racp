package app

type Config struct {
	Mode string `env:"MODE" default:"development" required:"true"`
}
