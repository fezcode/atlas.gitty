package config

import (
	"os"
	"path/filepath"

	"github.com/fezcode/go-piml"
)

type RepoEntry struct {
	Path string `piml:"path"`
}

type Config struct {
	Repositories []RepoEntry `piml:"repo,omitempty"`
}

func GetConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".atlas", "git.piml")
}

func Load() (*Config, error) {
	path := GetConfigPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &Config{}, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := piml.Unmarshal(content, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Save() error {
	path := GetConfigPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := piml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (c *Config) AddRepository(path string) {
	for _, r := range c.Repositories {
		if r.Path == path {
			return
		}
	}
	c.Repositories = append(c.Repositories, RepoEntry{Path: path})
}

func (c *Config) RemoveRepository(path string) {
	var newList []RepoEntry
	for _, r := range c.Repositories {
		if r.Path != path {
			newList = append(newList, r)
		}
	}
	c.Repositories = newList
}
