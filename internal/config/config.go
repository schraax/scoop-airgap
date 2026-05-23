package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Artifactory  ArtifactoryConfig `yaml:"artifactory"`
	Git          GitConfig         `yaml:"git"`
	Buckets      []BucketConfig    `yaml:"buckets"`
	Workers      int               `yaml:"workers"`
	// CooldownDays skips any app version whose manifest was committed to the
	// upstream bucket fewer than this many days ago. 0 disables the check.
	CooldownDays int               `yaml:"cooldown_days"`
}

type ArtifactoryConfig struct {
	// BaseURL is the root URL of Artifactory, e.g. https://artifactory.example.com/artifactory
	BaseURL  string `yaml:"base_url"`
	// Repo is the generic repository name in Artifactory
	Repo     string `yaml:"repo"`
	Username string `yaml:"username"`
	APIKey   string `yaml:"api_key"`
}

// GitConfig holds defaults that apply to every bucket's internal Git repo.
type GitConfig struct {
	Branch      string `yaml:"branch"`
	// AuthToken is embedded into HTTPS clone URLs as https://token@host/...
	AuthToken   string `yaml:"auth_token"`
	// LocalPathBase is the parent directory under which each bucket is cloned
	// as {LocalPathBase}/{bucket_name}. A system temp dir is used if empty.
	LocalPathBase string `yaml:"local_path_base"`
}

type BucketConfig struct {
	// Name is the bucket identifier, used as a path component in Artifactory
	// and as the clone subdirectory under GitConfig.LocalPathBase.
	Name string `yaml:"name"`
	// URL is the base raw-file URL for the public bucket, e.g.
	// https://raw.githubusercontent.com/ScoopInstaller/Main/master/bucket
	URL     string   `yaml:"url"`
	// RepoURL is the internal Git repo that will serve adapted manifests for
	// this bucket. Its layout must match a standard Scoop bucket repo so that
	// clients can add it with `scoop bucket add <name> <RepoURL>`.
	RepoURL string   `yaml:"repo_url"`
	Apps    []string `yaml:"apps"`
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
	if cfg.Git.Branch == "" {
		cfg.Git.Branch = "main"
	}

	return &cfg, nil
}
