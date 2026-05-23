package storage

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// artifactoryStore uploads to JFrog Artifactory.
// It supports both Basic auth (username + password) and the
// X-JFrog-Art-Api header (password only, no username required).
type artifactoryStore struct {
	baseURL  string
	username string
	password string
	http     *http.Client
}

func newArtifactory(baseURL, username, password string) Store {
	return &artifactoryStore{
		baseURL:  strings.TrimRight(baseURL, "/"),
		username: username,
		password: password,
		http:     &http.Client{Timeout: 10 * time.Minute},
	}
}

func (s *artifactoryStore) Exists(artifactPath string) (bool, error) {
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
	case http.StatusNotFound, http.StatusUnauthorized:
		return false, nil
	default:
		return false, fmt.Errorf("HEAD %s: HTTP %d", artifactPath, resp.StatusCode)
	}
}

func (s *artifactoryStore) Upload(artifactPath, localFile string) error {
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

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("PUT %s: HTTP %d", artifactPath, resp.StatusCode)
	}
	return nil
}

func (s *artifactoryStore) DownloadURL(artifactPath string) string {
	return s.url(artifactPath)
}

func (s *artifactoryStore) url(p string) string {
	return s.baseURL + "/" + strings.TrimLeft(p, "/")
}

func (s *artifactoryStore) auth(req *http.Request) {
	if s.username != "" {
		req.SetBasicAuth(s.username, s.password)
	} else if s.password != "" {
		// API-key-only auth — the header Artifactory accepts without a username
		req.Header.Set("X-JFrog-Art-Api", s.password)
	}
}
