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

A minimal image using the official Go builder and a scratch final layer:

```dockerfile
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY . .
RUN apk add --no-cache git \
 && go build -o /scoop-upstream ./cmd/scoop-upstream

FROM alpine:3.19
RUN apk add --no-cache git ca-certificates
COPY --from=build /scoop-upstream /usr/local/bin/scoop-upstream
ENTRYPOINT ["scoop-upstream"]
```

Build and push to your internal registry:

```bash
docker build -t registry.example.com/scoop-upstream:latest .
docker push registry.example.com/scoop-upstream:latest
```

{: .note }
The final image uses Alpine (not `scratch`) because `git` requires a C runtime and CA certificates are needed for HTTPS connections to Artifactory and your internal Git server.

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
