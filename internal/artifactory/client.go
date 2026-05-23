// Package artifactory provides a minimal Artifactory REST API client for
// checking existence and uploading binary artifacts.
package artifactory

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Client is a thin Artifactory REST API wrapper.
type Client struct {
	// baseURL is the Artifactory base URL including the repo name:
	//   https://artifactory.example.com/artifactory/my-repo
	baseURL  string
	username string
	apiKey   string
	http     *http.Client
}

// New creates a Client. artifBaseURL should be the full URL to the repository
// root (including repo name), e.g. https://artifactory.example.com/artifactory/scoop-mirror.
func New(artifBaseURL, username, apiKey string) *Client {
	return &Client{
		baseURL:  strings.TrimRight(artifBaseURL, "/"),
		username: username,
		apiKey:   apiKey,
		http:     &http.Client{Timeout: 10 * time.Minute},
	}
}

// Exists returns true if an artifact already exists at artifactPath.
func (c *Client) Exists(artifactPath string) (bool, error) {
	req, err := http.NewRequest(http.MethodHead, c.url(artifactPath), nil)
	if err != nil {
		return false, err
	}
	c.auth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return false, fmt.Errorf("HEAD %s: %w", artifactPath, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound, http.StatusUnauthorized:
		// 401 on a HEAD often means "not found" in Artifactory anonymous mode
		return false, nil
	default:
		return false, fmt.Errorf("HEAD %s: HTTP %d", artifactPath, resp.StatusCode)
	}
}

// Upload streams a local file to Artifactory at artifactPath.
func (c *Client) Upload(artifactPath, localFile string) error {
	f, err := os.Open(localFile)
	if err != nil {
		return fmt.Errorf("open %s: %w", localFile, err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", localFile, err)
	}

	req, err := http.NewRequest(http.MethodPut, c.url(artifactPath), f)
	if err != nil {
		return err
	}
	req.ContentLength = fi.Size()
	req.Header.Set("Content-Type", "application/octet-stream")
	c.auth(req)

	resp, err := c.http.Do(req)
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

// FullURL returns the downloadable URL for the given artifact path.
func (c *Client) FullURL(artifactPath string) string {
	return c.url(artifactPath)
}

func (c *Client) url(artifactPath string) string {
	return c.baseURL + "/" + strings.TrimLeft(artifactPath, "/")
}

func (c *Client) auth(req *http.Request) {
	if c.username != "" {
		req.SetBasicAuth(c.username, c.apiKey)
	} else if c.apiKey != "" {
		req.Header.Set("X-JFrog-Art-Api", c.apiKey)
	}
}
