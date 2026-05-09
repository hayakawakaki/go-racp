package config

// Config aggregates the application's environment-driven and YAML-driven settings.
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
