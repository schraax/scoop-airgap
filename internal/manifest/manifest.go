// Package manifest handles fetching, parsing, and rewriting Scoop bucket manifests.
// Only the url fields (top-level and per-architecture) are rewritten; all other
// fields — including hashes — are preserved verbatim, since the mirrored files are
// byte-identical to the originals.
package manifest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

// ArtifactRef describes one file that must be mirrored.
type ArtifactRef struct {
	// DownloadURL is the original URL without any Scoop fragment.
	DownloadURL string
	// Fragment is the Scoop rename suffix, e.g. "#/git-setup.exe". May be empty.
	Fragment string
	// Hash is the expected hash string as it appears in the manifest (e.g. "sha256:abc…").
	Hash string
	// ArtifactPath is the relative path inside the Artifactory repo:
	//   {bucket}/{app}/{version}/{filename}
	ArtifactPath string
	// ArtifactURL is the full URL clients will use to fetch this file.
	ArtifactURL string
}

// Fetch retrieves a manifest JSON from a public bucket raw URL and returns the
// parsed document plus the version string.
func Fetch(bucketBaseURL, app string) (map[string]interface{}, string, error) {
	rawURL := strings.TrimRight(bucketBaseURL, "/") + "/" + app + ".json"
	resp, err := http.Get(rawURL) //nolint:noctx
	if err != nil {
		return nil, "", fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, "", fmt.Errorf("manifest not found: %s", rawURL)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("fetch %s: HTTP %d", rawURL, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", rawURL, err)
	}

	var doc map[string]interface{}
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		return nil, "", fmt.Errorf("parse %s: %w", rawURL, err)
	}

	version, _ := doc["version"].(string)
	if version == "" {
		return nil, "", fmt.Errorf("manifest %s has no version field", rawURL)
	}

	return doc, version, nil
}

// Process rewrites all download URLs in doc to point at Artifactory and returns
// the list of artifacts that need to be mirrored.
// artifBaseURL is the full base URL of the Artifactory repo,
// e.g. https://artifactory.example.com/artifactory/scoop-mirror
func Process(doc map[string]interface{}, bucket, app, version, artifBaseURL string) ([]ArtifactRef, error) {
	base := strings.TrimRight(artifBaseURL, "/")
	var refs []ArtifactRef

	// Top-level url + hash
	if urls, hashes, ok := extractURLsHashes(doc, "url", "hash"); ok {
		newURLs := make([]string, len(urls))
		for i, u := range urls {
			ref, err := makeRef(u, hashes[i], bucket, app, version, base)
			if err != nil {
				return nil, err
			}
			refs = append(refs, ref)
			newURLs[i] = ref.ArtifactURL
		}
		setURLField(doc, "url", newURLs)
	}

	// Per-architecture urls
	if archRaw, ok := doc["architecture"]; ok {
		archMap, ok := archRaw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected type for 'architecture' field")
		}
		for _, arch := range []string{"64bit", "32bit", "arm64"} {
			archEntryRaw, ok := archMap[arch]
			if !ok {
				continue
			}
			archEntry, ok := archEntryRaw.(map[string]interface{})
			if !ok {
				continue
			}
			if urls, hashes, ok := extractURLsHashes(archEntry, "url", "hash"); ok {
				newURLs := make([]string, len(urls))
				for i, u := range urls {
					ref, err := makeRef(u, hashes[i], bucket, app, version, base)
					if err != nil {
						return nil, err
					}
					refs = append(refs, ref)
					newURLs[i] = ref.ArtifactURL
				}
				setURLField(archEntry, "url", newURLs)
			}
		}
	}

	return refs, nil
}

// Marshal serialises the (potentially rewritten) manifest back to indented JSON.
func Marshal(doc map[string]interface{}) ([]byte, error) {
	return json.MarshalIndent(doc, "", "    ")
}

// extractURLsHashes reads parallel url+hash fields from a map node.
// Both fields may be a single string or an array of strings.
// Returns (urls, hashes, found).
func extractURLsHashes(node map[string]interface{}, urlKey, hashKey string) ([]string, []string, bool) {
	urlRaw, ok := node[urlKey]
	if !ok {
		return nil, nil, false
	}

	urls := toStringSlice(urlRaw)
	if len(urls) == 0 {
		return nil, nil, false
	}

	var hashes []string
	if hashRaw, ok := node[hashKey]; ok {
		hashes = toStringSlice(hashRaw)
	}
	// Pad hashes with empty strings if fewer hashes than URLs.
	for len(hashes) < len(urls) {
		hashes = append(hashes, "")
	}

	return urls, hashes, true
}

func toStringSlice(v interface{}) []string {
	switch val := v.(type) {
	case string:
		return []string{val}
	case []interface{}:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// setURLField writes back a url field; uses a plain string if len==1.
func setURLField(node map[string]interface{}, key string, values []string) {
	if len(values) == 1 {
		node[key] = values[0]
	} else {
		iface := make([]interface{}, len(values))
		for i, v := range values {
			iface[i] = v
		}
		node[key] = iface
	}
}

// splitScoopURL separates the HTTP URL from the Scoop rename fragment (#/name).
func splitScoopURL(raw string) (downloadURL, fragment string) {
	idx := strings.Index(raw, "#")
	if idx == -1 {
		return raw, ""
	}
	return raw[:idx], raw[idx:]
}

func filenameFromURL(rawURL string) string {
	downloadURL, _ := splitScoopURL(rawURL)
	u, err := url.Parse(downloadURL)
	if err != nil {
		return path.Base(downloadURL)
	}
	name := path.Base(u.Path)
	if name == "." || name == "/" {
		name = "download"
	}
	return name
}

func makeRef(rawURL, hash, bucket, app, version, artifBase string) (ArtifactRef, error) {
	downloadURL, fragment := splitScoopURL(rawURL)
	if downloadURL == "" {
		return ArtifactRef{}, fmt.Errorf("empty URL in manifest")
	}

	filename := filenameFromURL(rawURL)
	// sanitise version for use as a path component
	safeVersion := strings.ReplaceAll(version, "/", "_")
	artifPath := path.Join(bucket, app, safeVersion, filename)
	artifURL := artifBase + "/" + artifPath + fragment

	return ArtifactRef{
		DownloadURL:  downloadURL,
		Fragment:     fragment,
		Hash:         hash,
		ArtifactPath: artifPath,
		ArtifactURL:  artifURL,
	}, nil
}
