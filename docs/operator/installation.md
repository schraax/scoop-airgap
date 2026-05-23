---
layout: default
title: Installation
parent: Operator Guide
nav_order: 1
---

# Installation
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Prerequisites

| Requirement | Notes |
|---|---|
| Go 1.22+ | Only needed for building from source |
| `git` | Must be on `PATH`; used for cloning/pushing bucket repos |
| Artifactory instance | A generic repository (any edition, including OSS) |
| Internal Git server | Gitea, GitLab, Bitbucket, Azure DevOps, or any standard Git host |
| Internet access | The host running scoop-upstream must be able to reach GitHub and software vendor download sites |

---

## Build from source

```bash
git clone https://github.com/yourorg/scoop-upstream.git
cd scoop-upstream
go build -o scoop-upstream ./cmd/scoop-upstream
```

The result is a single static binary with no runtime dependencies.

To cross-compile for a different architecture:

```bash
GOOS=linux GOARCH=amd64 go build -o scoop-upstream-linux-amd64 ./cmd/scoop-upstream
```

---

## Docker

The repository ships a `Dockerfile` that uses a two-stage build:

| Stage | Image | Purpose |
|---|---|---|
| build | `cgr.dev/chainguard/go` | Compiles a fully static binary (`CGO_ENABLED=0`) |
| runtime | `cgr.dev/chainguard/git` | Distroless image with git and CA certificates; no shell, no package manager |

[Chainguard](https://www.chainguard.dev) images are rebuilt daily from source with minimal installed packages, resulting in very few CVEs and a small attack surface.

Build and push to your internal registry:

```bash
docker build -t registry.example.com/scoop-upstream:latest .
docker push registry.example.com/scoop-upstream:latest
```

The runtime image runs as non-root (uid `65532`) by default. Ensure any host directories you bind-mount are writable by that uid:

```bash
mkdir -p /var/cache/scoop-upstream
chown -R 65532:65532 /var/cache/scoop-upstream
```

### Pinning image digests

The `Dockerfile` uses `:latest` tags for both Chainguard images. In production, pin to a specific digest for reproducible builds:

```bash
# Resolve current digests
docker buildx imagetools inspect cgr.dev/chainguard/go:latest     | grep Digest
docker buildx imagetools inspect cgr.dev/chainguard/git:latest    | grep Digest
```

Then replace `:latest` with `@sha256:<digest>` in the `Dockerfile`.

---

## Prepare Artifactory

1. Create a **Generic** repository. The name must match `artifactory.repo` in your config (e.g. `scoop-mirror`).
2. Create a service account or generate an API key with **deploy** permission on that repository.
3. Note the repository URL — it will be the value of `artifactory.base_url` plus `/scoop-mirror`.

---

## Prepare internal Git repos

Create one Git repository per bucket you intend to mirror. The repository can be completely empty; scoop-upstream will populate the `bucket/` directory on the first run.

```bash
# Example using the GitHub CLI — adapt for your Git server
gh repo create yourorg/scoop-main --private
gh repo create yourorg/scoop-extras --private
```

Grant the service account used by scoop-upstream **push** access to each repo.

If you are using an access token for HTTPS authentication, store it in an environment variable (e.g. `GIT_AUTH_TOKEN`) and reference it in the config. See [Configuration](configuration.md) for details.
