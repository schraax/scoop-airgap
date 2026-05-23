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
	// AuthToken is a personal access token embedded into HTTPS clone URLs.
	// Mutually exclusive with GitHubApp — if GitHubApp is set, AuthToken
	// is ignored.
	AuthToken string `yaml:"auth_token"`
	// GitHubApp configures GitHub App authentication as a more secure and
	// auditable alternative to a personal access token. When set, a
	// short-lived installation access token is fetched at startup and used
	// for all git operations and PR API calls.
	GitHubApp *GitHubAppConfig `yaml:"github_app"`
	// LocalPathBase is the parent directory under which each bucket is cloned
	// as {LocalPathBase}/{bucket_name}. A system temp dir is used if empty.
	LocalPathBase string `yaml:"local_path_base"`
	// PullRequest controls how manifest updates are delivered.
	// false (default): commit and push directly to the configured branch.
	// true: push changes to a new branch and open a pull request so a human
	// can review before the updated manifests go live.
	PullRequest bool `yaml:"pull_request"`
}

// GitHubAppConfig holds the credentials for a GitHub App installation.
type GitHubAppConfig struct {
	// AppID is the numeric GitHub App ID shown on the app's settings page.
	AppID int64 `yaml:"app_id"`
	// InstallationID is the numeric ID of the installation to authenticate as.
	// Find it in the URL when you click "Configure" on the app's installation.
	InstallationID int64 `yaml:"installation_id"`
	// PrivateKey is the RSA private key in PEM format. Use env-var expansion
	// to avoid storing the key in the config file:
	//   private_key: "${GITHUB_APP_PRIVATE_KEY}"
	PrivateKey string `yaml:"private_key"`
	// PrivateKeyPath is a path to a PEM file on disk. Use this instead of
	// PrivateKey when the key is mounted as a file (e.g. a Kubernetes secret).
	PrivateKeyPath string `yaml:"private_key_path"`
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
