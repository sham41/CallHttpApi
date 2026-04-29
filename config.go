package main

import (
	"fmt"
	"sync"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {

	// API configuration
	BaseUrl     string `yaml:"base_url" env-default:"https://test.site/"`
	InputPath   string `yaml:"input_path" env-default:""`
	OutputPath  string `yaml:"output_path" env-default:""`
	BearerToken string `yaml:"bearer_token" env-default:""`

	// batch processing configuration
	BatchSize    int    `yaml:"batch_size" env-default:"1000"`
	BatchWorkers int    `yaml:"batch_workers" env-default:"1"`
	BatchDelay   int    `yaml:"batch_delay" env-default:"0"`
	ItemsKey     string `yaml:"items_key" env-default:"items"`
}

var instance *Config
var once sync.Once

func GetConfig(path string) (*Config, error) {
	var err error
	once.Do(func() {
		instance = &Config{}
		if err = cleanenv.ReadConfig(path, instance); err != nil {
			desc, _ := cleanenv.GetDescription(instance, nil)
			err = fmt.Errorf("%s; %s", err, desc)
			instance = nil
		}
	})
	return instance, err
}
