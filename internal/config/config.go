package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Storage      StorageConfig `yaml:"storage"`
	Git          GitConfig     `yaml:"git"`
	Buckets      []BucketConfig `yaml:"buckets"`
	Workers      int            `yaml:"workers"`
	// CooldownDays skips any app version whose manifest was committed to the
	// upstream bucket fewer than this many days ago. 0 disables the check.
	CooldownDays int            `yaml:"cooldown_days"`
}

type StorageConfig struct {
	// Type selects the backend: "artifactory" (default) or "generic".
	// "generic" uses plain Basic auth and is compatible with Nexus OSS
	// raw repositories, nginx WebDAV, Caddy, and similar HTTP file servers.
	Type     string `yaml:"type"`
	// BaseURL is the root URL of the storage server, without the repo name.
	// Artifactory example: https://artifactory.example.com/artifactory
	// Nexus example:       http://nexus:8081/repository  (omit trailing slash)
	BaseURL  string `yaml:"base_url"`
	// Repo is the repository / bucket name inside the storage server.
	Repo     string `yaml:"repo"`
	Username string `yaml:"username"`
	// Password is the Artifactory API key, Nexus password, or HTTP Basic auth
	// password, depending on the backend type.
	Password string `yaml:"password"`
}

// GitConfig holds defaults that apply to every bucket's internal Git repo.
type GitConfig struct {
	Branch string `yaml:"branch"`
	// AuthToken is embedded into HTTPS clone URLs as https://token@host/...
	AuthToken string `yaml:"auth_token"`
	// LocalPathBase is the parent directory under which each bucket is cloned
	// as {LocalPathBase}/{bucket_name}. A system temp dir is used if empty.
	LocalPathBase string `yaml:"local_path_base"`
	// PullRequest controls how manifest updates are delivered.
	// false (default): commit and push directly to the configured branch.
	// true: push changes to a new branch and open a pull request so a human
	// can review before the updated manifests go live.
	PullRequest bool `yaml:"pull_request"`
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
