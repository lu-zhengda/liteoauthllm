package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Port    int  `yaml:"port"`
	Verbose bool `yaml:"verbose"`
}

func Default() Config {
	return Config{
		Port:    8639,
		Verbose: false,
	}
}

func LoadFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config file: %w", err)
	}

	return cfg, nil
}

func Merge(base, override Config) Config {
	result := base
	if override.Port != 0 {
		result.Port = override.Port
	}
	if override.Verbose {
		result.Verbose = true
	}
	return result
}
