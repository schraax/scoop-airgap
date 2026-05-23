// Package download fetches URLs to temporary files and verifies their hashes.
package download

import (
	"crypto/md5"  //nolint:gosec
	"crypto/sha1" //nolint:gosec
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"strings"
)

// ToTemp downloads url into a new temporary file, verifies expectedHash (may be
// empty to skip verification), and returns the temp file path.
// The caller is responsible for removing the file when done.
func ToTemp(rawURL, expectedHash string) (string, error) {
	tmp, err := os.CreateTemp("", "scoop-upstream-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer tmp.Close()

	resp, err := http.Get(rawURL) //nolint:noctx
	if err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("GET %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("GET %s: HTTP %d", rawURL, resp.StatusCode)
	}

	h, hashType, err := newHasher(expectedHash)
	if err != nil {
		os.Remove(tmp.Name())
		return "", err
	}

	var w io.Writer = tmp
	if h != nil {
		w = io.MultiWriter(tmp, h)
	}

	if _, err := io.Copy(w, resp.Body); err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("download %s: %w", rawURL, err)
	}

	if h != nil {
		got := hex.EncodeToString(h.Sum(nil))
		want := expectedHashValue(expectedHash)
		if !strings.EqualFold(got, want) {
			os.Remove(tmp.Name())
			return "", fmt.Errorf("hash mismatch for %s (%s): got %s, want %s", rawURL, hashType, got, want)
		}
	}

	return tmp.Name(), nil
}

// newHasher returns the appropriate hash.Hash for the Scoop hash string.
// Scoop hash formats: bare 64-char hex (sha256), or "sha256:hex", "sha512:hex",
// "sha1:hex", "md5:hex".
func newHasher(h string) (hash.Hash, string, error) {
	if h == "" {
		return nil, "", nil
	}

	lower := strings.ToLower(h)
	switch {
	case strings.HasPrefix(lower, "sha512:"):
		return sha512.New(), "sha512", nil
	case strings.HasPrefix(lower, "sha256:"):
		return sha256.New(), "sha256", nil
	case strings.HasPrefix(lower, "sha1:"):
		return sha1.New(), "sha1", nil //nolint:gosec
	case strings.HasPrefix(lower, "md5:"):
		return md5.New(), "md5", nil //nolint:gosec
	default:
		// bare hex: length determines algorithm
		val := strings.TrimSpace(h)
		switch len(val) {
		case 128:
			return sha512.New(), "sha512", nil
		case 64:
			return sha256.New(), "sha256", nil
		case 40:
			return sha1.New(), "sha1", nil //nolint:gosec
		case 32:
			return md5.New(), "md5", nil //nolint:gosec
		default:
			return nil, "", fmt.Errorf("unrecognised hash format: %q", h)
		}
	}
}

// expectedHashValue strips the "algo:" prefix from a Scoop hash string.
func expectedHashValue(h string) string {
	if idx := strings.Index(h, ":"); idx != -1 {
		return h[idx+1:]
	}
	return h
}
