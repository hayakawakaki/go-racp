package config

type Config struct {
	Env *EnvConfig
}

func NewConfig() (*Config, error) {
	env, err := ProcessEnv()
	if err != nil {
		return nil, err
	}

	return &Config{
		Env: env,
	}, nil
}
