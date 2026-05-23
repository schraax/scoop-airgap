package storage

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// genericStore uploads to any HTTP server that accepts PUT for upload and HEAD
// for existence checks, authenticated with HTTP Basic auth.
//
// Compatible backends include:
//   - Sonatype Nexus Repository OSS — raw (hosted) repository
//   - nginx with the ngx_http_dav_module (WebDAV)
//   - Caddy with the file server + WebDAV plugin
//   - Apache httpd with mod_dav
//
// The URL structure must follow the same convention as Artifactory:
//
//	{base_url}/{repo}/{bucket}/{app}/{version}/{filename}
//
// so that Scoop manifests can be rewritten with a simple base URL swap when
// switching between backends.
type genericStore struct {
	baseURL  string
	username string
	password string
	http     *http.Client
}

func newGeneric(baseURL, username, password string) Store {
	return &genericStore{
		baseURL:  strings.TrimRight(baseURL, "/"),
		username: username,
		password: password,
		http:     &http.Client{Timeout: 10 * time.Minute},
	}
}

func (s *genericStore) Exists(artifactPath string) (bool, error) {
	req, err := http.NewRequest(http.MethodHead, s.url(artifactPath), nil)
	if err != nil {
		return false, err
	}
	s.auth(req)

	resp, err := s.http.Do(req)
	if err != nil {
		return false, fmt.Errorf("HEAD %s: %w", artifactPath, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	case http.StatusUnauthorized, http.StatusForbidden:
		// Surface auth errors explicitly so operators don't spend time
		// wondering why every artifact is re-uploaded on every run.
		return false, fmt.Errorf("HEAD %s: HTTP %d (check storage credentials)", artifactPath, resp.StatusCode)
	default:
		return false, fmt.Errorf("HEAD %s: HTTP %d", artifactPath, resp.StatusCode)
	}
}

func (s *genericStore) Upload(artifactPath, localFile string) error {
	f, err := os.Open(localFile)
	if err != nil {
		return fmt.Errorf("open %s: %w", localFile, err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", localFile, err)
	}

	req, err := http.NewRequest(http.MethodPut, s.url(artifactPath), f)
	if err != nil {
		return err
	}
	req.ContentLength = fi.Size()
	req.Header.Set("Content-Type", "application/octet-stream")
	s.auth(req)

	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("PUT %s: %w", artifactPath, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck

	// WebDAV servers return 201 Created; some return 204 No Content on overwrite.
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		return nil
	default:
		return fmt.Errorf("PUT %s: HTTP %d", artifactPath, resp.StatusCode)
	}
}

func (s *genericStore) DownloadURL(artifactPath string) string {
	return s.url(artifactPath)
}

func (s *genericStore) url(p string) string {
	return s.baseURL + "/" + strings.TrimLeft(p, "/")
}

func (s *genericStore) auth(req *http.Request) {
	if s.username != "" || s.password != "" {
		req.SetBasicAuth(s.username, s.password)
	}
}
