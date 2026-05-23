package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Artifactory ArtifactoryConfig `yaml:"artifactory"`
	BucketRepo  BucketRepoConfig  `yaml:"bucket_repo"`
	Buckets     []BucketConfig    `yaml:"buckets"`
	Workers     int               `yaml:"workers"`
}

type ArtifactoryConfig struct {
	// BaseURL is the root URL of Artifactory, e.g. https://artifactory.example.com/artifactory
	BaseURL  string `yaml:"base_url"`
	// Repo is the generic repository name in Artifactory
	Repo     string `yaml:"repo"`
	Username string `yaml:"username"`
	APIKey   string `yaml:"api_key"`
}

type BucketRepoConfig struct {
	// URL is the internal Git repository for adapted bucket manifests
	URL       string `yaml:"url"`
	Branch    string `yaml:"branch"`
	// LocalPath is where the repo is cloned on disk (temp dir if empty)
	LocalPath string `yaml:"local_path"`
	// AuthToken is embedded into HTTPS clone URL as https://token@host/...
	AuthToken string `yaml:"auth_token"`
}

type BucketConfig struct {
	// Name is the bucket identifier, used as a path component in Artifactory
	Name string `yaml:"name"`
	// URL is the base raw-file URL for the public bucket, e.g.
	// https://raw.githubusercontent.com/ScoopInstaller/Main/master/bucket
	URL  string   `yaml:"url"`
	Apps []string `yaml:"apps"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	// expand environment variables so secrets can live outside the config file
	data = []byte(os.ExpandEnv(string(data)))

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}
	if cfg.BucketRepo.Branch == "" {
		cfg.BucketRepo.Branch = "main"
	}

	return &cfg, nil
}
