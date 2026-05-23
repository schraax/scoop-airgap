// Package storage provides an abstraction over binary artifact stores.
// Two implementations are provided:
//   - "artifactory": JFrog Artifactory (supports X-JFrog-Art-Api key auth)
//   - "generic":     any HTTP server that accepts PUT/HEAD with Basic auth
//     (Sonatype Nexus OSS raw repositories, nginx with WebDAV, Caddy, …)
package storage

import (
	"fmt"

	"github.com/andreasj/scoop-upstream/internal/config"
)

// Store is the interface every backend must satisfy.
type Store interface {
	// Exists reports whether an artifact is already present at artifactPath.
	Exists(artifactPath string) (bool, error)
	// Upload streams localFile to artifactPath in the store.
	Upload(artifactPath, localFile string) error
	// DownloadURL returns the URL that Scoop clients use to fetch this artifact.
	DownloadURL(artifactPath string) string
}

// New returns the Store implementation selected by cfg.Type.
// An empty type defaults to "artifactory" for backward compatibility.
func New(cfg config.StorageConfig) (Store, error) {
	base := cfg.BaseURL + "/" + cfg.Repo

	switch cfg.Type {
	case "", "artifactory":
		return newArtifactory(base, cfg.Username, cfg.Password), nil
	case "generic":
		return newGeneric(base, cfg.Username, cfg.Password), nil
	default:
		return nil, fmt.Errorf("unknown storage type %q: must be \"artifactory\" or \"generic\"", cfg.Type)
	}
}
