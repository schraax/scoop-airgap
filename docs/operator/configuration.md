---
layout: default
title: Configuration
parent: Operator Guide
nav_order: 2
---

# Configuration
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Config file

scoop-upstream reads a YAML file (default: `config.yaml` in the working directory). Pass a different path with `-config /path/to/config.yaml`.

All string values support `${ENV_VAR}` expansion so that secrets never need to be written into the file.

A fully annotated example is provided in [`config.example.yaml`](https://github.com/yourorg/scoop-upstream/blob/master/config.example.yaml) at the root of the repository.

---

## `storage` block

```yaml
storage:
  type: artifactory          # "artifactory" (default) or "generic"
  base_url: https://artifactory.example.com/artifactory
  repo: scoop-mirror
  username: ${STORAGE_USER}  # omit for Artifactory API-key-only auth
  password: ${STORAGE_PASSWORD}
```

| Field | Required | Description |
|---|---|---|
| `type` | no | `artifactory` (default) or `generic`. See below. |
| `base_url` | yes | Root URL of the storage server, **without** the repository name and without a trailing slash. |
| `repo` | yes | Repository name inside the storage server. |
| `username` | no | Username for Basic auth. Omit when using Artifactory API-key-only auth. |
| `password` | yes | Artifactory API key, Nexus password, or HTTP Basic auth password. Use `${ENV_VAR}` to avoid storing it in the file. |

### Backend types

#### `artifactory` (default)

JFrog Artifactory. When `username` is omitted the password is sent as the
`X-JFrog-Art-Api` header. When `username` is set, Basic auth is used — both
methods are accepted by Artifactory.

#### `generic`

Any HTTP server that accepts `PUT` for upload and `HEAD` for existence checks,
authenticated with HTTP Basic auth. Tested backends:

| Product | Setup notes |
|---|---|
| **Sonatype Nexus Repository OSS** | Create a *raw (hosted)* repository. Use the repository URL as `base_url` with `repository` in the path: `http://nexus:8081/repository`. |
| **nginx** | Enable `ngx_http_dav_module`. Add `dav_methods PUT;` and `auth_basic` to the location block. |
| **Caddy** | Use the `file_server` directive with `browse` disabled, and `basicauth` for write protection. |

### Storage URL structure

Binaries are stored at the same path layout regardless of backend:

```
{base_url}/{repo}/{bucket}/{app}/{version}/{filename}
```

Example:

```
https://artifactory.example.com/artifactory/scoop-mirror/main/git/2.43.0/Git-2.43.0-64-bit.exe
http://nexus:8081/repository/scoop-mirror/main/git/2.43.0/Git-2.43.0-64-bit.exe
```

This means switching backends only requires updating `type`, `base_url`, and credentials — the rewritten manifest URLs follow automatically.

---

## `git` block

```yaml
git:
  branch: main
  auth_token: ${GIT_AUTH_TOKEN}
  local_path_base: /var/cache/scoop-upstream
```

| Field | Required | Description |
|---|---|---|
| `branch` | no | Branch to clone and push to. Defaults to `main`. |
| `auth_token` | no | Personal access token or deploy token. When set it is embedded in the HTTPS clone URL as `https://token@host/path`. Works with Gitea, GitLab, GitHub, Azure DevOps, and Bitbucket. |
| `local_path_base` | no | Parent directory for local clones. Each bucket is cloned as `{local_path_base}/{bucket_name}`. If omitted, a system temp directory is used (the clone is lost after each run, meaning a fresh clone on every invocation). |

{: .note }
If using SSH authentication instead of a token, leave `auth_token` empty and ensure the host running scoop-upstream has the correct SSH key loaded. The `git` binary on `PATH` will handle SSH authentication transparently.

---

## `buckets` list

```yaml
buckets:
  - name: main
    url: https://raw.githubusercontent.com/ScoopInstaller/Main/master/bucket
    repo_url: https://git.example.com/scoop/main.git
    apps:
      - git
      - curl
      - jq

  - name: extras
    url: https://raw.githubusercontent.com/ScoopInstaller/Extras/master/bucket
    repo_url: https://git.example.com/scoop/extras.git
    apps:
      - vscode
```

| Field | Required | Description |
|---|---|---|
| `name` | yes | Short identifier for this bucket. Used as a path component in Artifactory and as the clone subdirectory under `git.local_path_base`. Must be unique. |
| `url` | yes | Base URL for fetching raw manifest JSON from the public upstream bucket. The tool appends `/{app}.json` to form the full URL. |
| `repo_url` | yes | HTTPS or SSH URL of the internal Git repository that will host the adapted manifests for this bucket. Must already exist on the Git server. |
| `apps` | yes | List of app names to mirror. Each name must correspond to a `{name}.json` file in the upstream bucket. |

### Finding the raw URL for a bucket

For the official Scoop buckets:

| Bucket | Raw URL |
|---|---|
| main | `https://raw.githubusercontent.com/ScoopInstaller/Main/master/bucket` |
| extras | `https://raw.githubusercontent.com/ScoopInstaller/Extras/master/bucket` |
| versions | `https://raw.githubusercontent.com/ScoopInstaller/Versions/master/bucket` |
| java | `https://raw.githubusercontent.com/ScoopInstaller/Java/master/bucket` |

For any other bucket, the pattern is:
`https://raw.githubusercontent.com/{owner}/{repo}/{branch}/bucket`

---

## `workers`

```yaml
workers: 4
```

Number of concurrent download/upload workers. Defaults to `4`. Increase for faster syncs when bandwidth allows; reduce if Artifactory or the download sources impose rate limits.

---

## `cooldown_days`

```yaml
cooldown_days: 3
```

When set to a positive integer, scoop-upstream skips any app version whose manifest was committed to the upstream bucket fewer than this many days ago. This prevents brand-new releases — which may contain regressions or be quickly superseded by a hotfix — from entering the internal mirror immediately.

How it works: for each app, scoop-upstream queries the GitHub REST API for the most recent commit that touched `bucket/{app}.json`. If that commit is younger than `cooldown_days`, the app is skipped for this run. On the next scheduled run (once the cooldown has elapsed) it will be picked up automatically.

| Value | Behaviour |
|---|---|
| `0` (default) | No cooldown check; all available versions are mirrored immediately. |
| `3` | Wait three days after a new version appears before mirroring it. |
| `7` | Wait one week — a common choice for production environments. |

**Limitations:**
- Only GitHub-hosted buckets (`raw.githubusercontent.com`) are supported. For other hosts the check is silently skipped and the app is mirrored normally.
- The GitHub REST API allows 60 unauthenticated requests per hour. With many apps this limit can be reached. Set the `GITHUB_TOKEN` environment variable (any valid personal access token, no specific scopes required) to raise the limit to 5 000 requests per hour.
- If the API call fails for any reason, a warning is logged and the app is mirrored anyway (fail-open).

---

## Environment variables

All secrets should be passed as environment variables. Recommended names:

| Variable | Used for |
|---|---|
| `STORAGE_USER` | Storage server username |
| `STORAGE_PASSWORD` | Artifactory API key, Nexus password, or HTTP Basic auth password |
| `GIT_AUTH_TOKEN` | Git server personal access token |

---

## Complete minimal example

```yaml
storage:
  base_url: https://artifactory.example.com/artifactory
  repo: scoop-mirror
  password: ${STORAGE_PASSWORD}

git:
  branch: main
  auth_token: ${GIT_AUTH_TOKEN}
  local_path_base: /var/cache/scoop-upstream

buckets:
  - name: main
    url: https://raw.githubusercontent.com/ScoopInstaller/Main/master/bucket
    repo_url: https://git.example.com/scoop/main.git
    apps:
      - git
      - curl
      - jq
      - 7zip
```
