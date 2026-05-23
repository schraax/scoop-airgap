package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/schraax/scoop-airgap/internal/config"
	"github.com/schraax/scoop-airgap/internal/download"
	"github.com/schraax/scoop-airgap/internal/gitrepo"
	"github.com/schraax/scoop-airgap/internal/manifest"
	"github.com/schraax/scoop-airgap/internal/storage"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	dryRun := flag.Bool("dry-run", false, "print what would be done without making changes")
	filterApp := flag.String("app", "", "sync only this app (optional)")
	force := flag.Bool("force", false, "re-upload even if artifact already exists in the store")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr,
				"Error: config file not found: %s\n\n"+
					"Create a config.yaml based on config.example.yaml and pass its path with -config.\n"+
					"Full reference: https://github.com/schraax/scoop-airgap/blob/master/docs/operator/configuration.md\n",
				*configPath)
			os.Exit(1)
		}
		log.Fatalf("load config: %v", err)
	}

	if err := run(cfg, *dryRun, *filterApp, *force); err != nil {
		log.Fatalf("sync failed: %v", err)
	}
}

func run(cfg *config.Config, dryRun bool, filterApp string, force bool) error {
	store, err := storage.New(cfg.Storage)
	if err != nil {
		return fmt.Errorf("init storage: %w", err)
	}

	artifBaseURL := cfg.Storage.BaseURL + "/" + cfg.Storage.Repo

	// Open one Repo handle per bucket; each has its own internal Git repo.
	type bucketRepo struct {
		cfg  config.BucketConfig
		repo *gitrepo.Repo
	}
	var bucketRepos []bucketRepo

	for _, b := range cfg.Buckets {
		if b.RepoURL == "" {
			return fmt.Errorf("bucket %q: repo_url is required", b.Name)
		}
		localPath := localPathFor(cfg.Git.LocalPathBase, b.Name)
		repo, err := gitrepo.New(b.RepoURL, cfg.Git.Branch, localPath, cfg.Git.AuthToken)
		if err != nil {
			return fmt.Errorf("init repo for bucket %s: %w", b.Name, err)
		}
		if !dryRun {
			log.Printf("cloning/pulling bucket repo %s → %s", b.RepoURL, localPath)
			if err := repo.EnsureReady(); err != nil {
				return fmt.Errorf("prepare repo for bucket %s: %w", b.Name, err)
			}
		}
		bucketRepos = append(bucketRepos, bucketRepo{b, repo})
	}

	// Build the flat job list.
	type job struct {
		bucket config.BucketConfig
		app    string
	}
	var jobs []job
	for _, br := range bucketRepos {
		for _, app := range br.cfg.Apps {
			if filterApp != "" && app != filterApp {
				continue
			}
			jobs = append(jobs, job{br.cfg, app})
		}
	}

	type manifestResult struct {
		bucket string
		app    string
		data   []byte
	}

	jobCh := make(chan job, len(jobs))
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)

	type result struct {
		bucket, app string
		err         error
	}
	resultCh := make(chan result, len(jobs))
	manifestCh := make(chan manifestResult, len(jobs))

	workers := cfg.Workers
	if workers < 1 {
		workers = 1
	}

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobCh {
				data, err := syncApp(j.bucket, j.app, artifBaseURL, store, dryRun, force, cfg.CooldownDays)
				resultCh <- result{j.bucket.Name, j.app, err}
				if err == nil && data != nil {
					manifestCh <- manifestResult{j.bucket.Name, j.app, data}
				}
			}
		}()
	}
	wg.Wait()
	close(resultCh)
	close(manifestCh)

	var errCount int
	for r := range resultCh {
		if r.err != nil {
			log.Printf("ERROR %s/%s: %v", r.bucket, r.app, r.err)
			errCount++
		}
	}

	if dryRun {
		log.Println("[dry-run] skipping Git commit/push")
		if errCount > 0 {
			return fmt.Errorf("%d app(s) had errors", errCount)
		}
		return nil
	}

	// Write manifests into their respective bucket repos.
	repoByName := make(map[string]*gitrepo.Repo, len(bucketRepos))
	for _, br := range bucketRepos {
		repoByName[br.cfg.Name] = br.repo
	}
	for m := range manifestCh {
		repo := repoByName[m.bucket]
		if err := repo.WriteManifest(m.app, m.data); err != nil {
			log.Printf("ERROR write manifest %s/%s: %v", m.bucket, m.app, err)
			errCount++
		}
	}

	// Commit and push each bucket repo independently.
	for _, br := range bucketRepos {
		log.Printf("committing and pushing bucket %s", br.cfg.Name)
		if err := br.repo.CommitAndPush("sync: mirror scoop manifests to Artifactory"); err != nil {
			log.Printf("ERROR git push for bucket %s: %v", br.cfg.Name, err)
			errCount++
		}
	}

	if errCount > 0 {
		return fmt.Errorf("%d error(s) during sync", errCount)
	}
	return nil
}

// localPathFor returns the clone directory for a bucket.
// If base is empty a system temp dir is used.
func localPathFor(base, bucketName string) string {
	if base == "" {
		return "" // gitrepo.New will create a temp dir
	}
	return filepath.Join(base, bucketName)
}

// syncApp fetches the public manifest, mirrors all artifacts to the store,
// and returns the rewritten manifest JSON. Returns nil data on dry-run.
func syncApp(
	bucket config.BucketConfig,
	app string,
	artifBaseURL string,
	store storage.Store,
	dryRun bool,
	force bool,
	cooldownDays int,
) ([]byte, error) {
	doc, version, err := manifest.Fetch(bucket.URL, app)
	if err != nil {
		return nil, err
	}

	if cooldownDays > 0 {
		age, err := manifest.CommitAge(bucket.URL, app)
		if err != nil {
			log.Printf("WARN %s/%s: cooldown check failed, proceeding anyway: %v", bucket.Name, app, err)
		} else if age > 0 && age < time.Duration(cooldownDays)*24*time.Hour {
			log.Printf("skip (cooldown %dd, age %.1fd): %s/%s@%s",
				cooldownDays, age.Hours()/24, bucket.Name, app, version)
			return nil, nil
		}
	}

	refs, err := manifest.Process(doc, bucket.Name, app, version, artifBaseURL)
	if err != nil {
		return nil, fmt.Errorf("process manifest: %w", err)
	}

	for _, ref := range refs {
		label := fmt.Sprintf("%s/%s@%s → %s", bucket.Name, app, version, ref.ArtifactPath)

		if dryRun {
			log.Printf("[dry-run] would mirror %s", label)
			continue
		}

		if !force {
			exists, err := store.Exists(ref.ArtifactPath)
			if err != nil {
				log.Printf("WARN check exists %s: %v", ref.ArtifactPath, err)
			}
			if exists {
				log.Printf("skip (already in store): %s", label)
				continue
			}
		}

		log.Printf("downloading %s", ref.DownloadURL)
		tmpFile, err := download.ToTemp(ref.DownloadURL, ref.Hash)
		if err != nil {
			return nil, fmt.Errorf("download %s: %w", ref.DownloadURL, err)
		}

		log.Printf("uploading %s", label)
		if err := store.Upload(ref.ArtifactPath, tmpFile); err != nil {
			os.Remove(tmpFile)
			return nil, fmt.Errorf("upload %s: %w", ref.ArtifactPath, err)
		}
		os.Remove(tmpFile)
	}

	if dryRun {
		return nil, nil
	}

	data, err := manifest.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}
	return data, nil
}
