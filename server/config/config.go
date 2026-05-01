package config

type Config struct {
	Env *EnvConfig
}

func NewConfig() *Config {
	return &Config{
		Env: ProcessEnv(),
	}
}
