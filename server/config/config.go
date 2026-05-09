package config

type Config struct {
	Env *EnvConfig
	App *AppConfig
}

func NewConfig() *Config {
	return &Config{
		Env: ProcessEnv(),
		App: ProcessAppConfig(),
	}
}
