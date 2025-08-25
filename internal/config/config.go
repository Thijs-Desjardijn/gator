package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	DataBaseUrl     string `json:"db_url"`
	CurrentUserName string `json:"current_user_name"`
}

const configFileName = ".gatorconfig.json"

func Read() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, err
	}

	fullpath := filepath.Join(home, configFileName)
	file, err := os.Open(fullpath)
	if err != nil {
		return Config{}, err
	}

	var config Config
	err = json.NewDecoder(file).Decode(&config)
	if err != nil {
		return Config{}, err
	}

	return config, nil
}

func (c *Config) SetUser(username string) error {
	c.CurrentUserName = username
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	fullpath := filepath.Join(home, configFileName)
	file, err := os.Create(fullpath)
	if err != nil {
		return err
	}
	err = json.NewEncoder(file).Encode(c)
	if err != nil {
		return err
	}
	return nil
}
