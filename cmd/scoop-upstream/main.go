package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/andreasj/scoop-upstream/internal/artifactory"
	"github.com/andreasj/scoop-upstream/internal/config"
	"github.com/andreasj/scoop-upstream/internal/download"
	"github.com/andreasj/scoop-upstream/internal/gitrepo"
	"github.com/andreasj/scoop-upstream/internal/manifest"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	dryRun := flag.Bool("dry-run", false, "print what would be done without making changes")
	filterApp := flag.String("app", "", "sync only this app (optional)")
	force := flag.Bool("force", false, "re-upload even if artifact already exists in Artifactory")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if err := run(cfg, *dryRun, *filterApp, *force); err != nil {
		log.Fatalf("sync failed: %v", err)
	}
}

func run(cfg *config.Config, dryRun bool, filterApp string, force bool) error {
	artifClient := artifactory.New(
		cfg.Artifactory.BaseURL+"/"+cfg.Artifactory.Repo,
		cfg.Artifactory.Username,
		cfg.Artifactory.APIKey,
	)

	repo, err := gitrepo.New(
		cfg.BucketRepo.URL,
		cfg.BucketRepo.Branch,
		cfg.BucketRepo.LocalPath,
		cfg.BucketRepo.AuthToken,
	)
	if err != nil {
		return fmt.Errorf("init bucket repo: %w", err)
	}

	if !dryRun {
		log.Printf("cloning/pulling bucket repo %s", cfg.BucketRepo.URL)
		if err := repo.EnsureReady(); err != nil {
			return fmt.Errorf("prepare bucket repo: %w", err)
		}
	}

	artifBaseURL := cfg.Artifactory.BaseURL + "/" + cfg.Artifactory.Repo

	type job struct {
		bucket config.BucketConfig
		app    string
	}

	var jobs []job
	for _, b := range cfg.Buckets {
		for _, app := range b.Apps {
			if filterApp != "" && app != filterApp {
				continue
			}
			jobs = append(jobs, job{b, app})
		}
	}

	type result struct {
		bucket, app string
		err         error
	}

	jobCh := make(chan job, len(jobs))
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)

	resultCh := make(chan result, len(jobs))
	var wg sync.WaitGroup

	workers := cfg.Workers
	if workers < 1 {
		workers = 1
	}

	// results also carry the rewritten manifest; collect them for the Git step
	type manifestResult struct {
		bucket, app string
		data        []byte
	}
	manifestCh := make(chan manifestResult, len(jobs))

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobCh {
				data, err := syncApp(j.bucket, j.app, artifBaseURL, artifClient, dryRun, force)
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
		return nil
	}

	for m := range manifestCh {
		if err := repo.WriteManifest(m.bucket, m.app, m.data); err != nil {
			log.Printf("ERROR write manifest %s/%s: %v", m.bucket, m.app, err)
			errCount++
		}
	}

	log.Println("committing and pushing mirrored manifests")
	if err := repo.CommitAndPush("sync: mirror scoop manifests to Artifactory"); err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	if errCount > 0 {
		return fmt.Errorf("%d app(s) failed to sync", errCount)
	}
	return nil
}

// syncApp fetches the public manifest, mirrors all artifacts to Artifactory,
// and returns the rewritten manifest JSON. Returns nil data on dry-run.
func syncApp(
	bucket config.BucketConfig,
	app string,
	artifBaseURL string,
	artif *artifactory.Client,
	dryRun bool,
	force bool,
) ([]byte, error) {
	doc, version, err := manifest.Fetch(bucket.URL, app)
	if err != nil {
		return nil, err
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
			exists, err := artif.Exists(ref.ArtifactPath)
			if err != nil {
				log.Printf("WARN check exists %s: %v", ref.ArtifactPath, err)
			}
			if exists {
				log.Printf("skip (already in Artifactory): %s", label)
				continue
			}
		}

		log.Printf("downloading %s", ref.DownloadURL)
		tmpFile, err := download.ToTemp(ref.DownloadURL, ref.Hash)
		if err != nil {
			return nil, fmt.Errorf("download %s: %w", ref.DownloadURL, err)
		}

		log.Printf("uploading %s", label)
		if err := artif.Upload(ref.ArtifactPath, tmpFile); err != nil {
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
