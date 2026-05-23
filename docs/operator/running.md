---
layout: default
title: Running
parent: Operator Guide
nav_order: 3
---

# Running
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Command-line flags

```
Usage of scoop-upstream:
  -config string
      path to config file (default "config.yaml")
  -app string
      sync only this app (optional)
  -dry-run
      print what would be done without making changes
  -force
      re-upload even if artifact already exists in Artifactory
```

---

## One-shot run

```bash
export ARTIFACTORY_API_KEY=...
export GIT_AUTH_TOKEN=...

scoop-upstream -config /etc/scoop-upstream/config.yaml
```

On a clean run, scoop-upstream:

1. Clones each bucket's internal Git repo (or pulls if already cloned).
2. For each app in the allowlist, fetches the public manifest, downloads any binaries not already in Artifactory, uploads them, and rewrites the manifest.
3. Commits and pushes all updated manifests to the internal Git repos.

Subsequent runs skip binaries that already exist in Artifactory (`HEAD` check), so only new versions are transferred.

---

## Dry run

Use `-dry-run` to preview what would be synced without making any changes to Artifactory or the Git repos:

```bash
scoop-upstream -config config.yaml -dry-run
```

Output example:

```
[dry-run] would mirror main/git@2.43.0 → main/git/2.43.0/Git-2.43.0-64-bit.exe
[dry-run] would mirror extras/vscode@1.88.0 → extras/vscode/1.88.0/VSCodeSetup-x64-1.88.0.exe
[dry-run] skipping Git commit/push
```

---

## Syncing a single app

To sync one app without touching others — useful for testing or urgent updates:

```bash
scoop-upstream -config config.yaml -app git
```

The `-app` flag matches by app name across all configured buckets.

---

## Force re-upload

By default, scoop-upstream skips artifacts already present in Artifactory. Use `-force` to re-download and re-upload regardless:

```bash
scoop-upstream -config config.yaml -force -app curl
```

This is useful when a binary in Artifactory was corrupted or accidentally deleted.

---

## Scheduled operation (cron)

Add a crontab entry on the Linux host to run scoop-upstream daily:

```cron
# /etc/cron.d/scoop-upstream
0 3 * * * svc-scoop /usr/local/bin/scoop-upstream -config /etc/scoop-upstream/config.yaml >> /var/log/scoop-upstream.log 2>&1
```

The tool is safe to run while clients are actively downloading; Artifactory serves existing files during the upload phase.

---

## Docker / container

### Single run

```bash
docker run --rm \
  -v /etc/scoop-upstream:/config:ro \
  -v /var/cache/scoop-upstream:/cache \
  -e ARTIFACTORY_API_KEY \
  -e GIT_AUTH_TOKEN \
  registry.example.com/scoop-upstream:latest \
  -config /config/config.yaml
```

### Kubernetes CronJob

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: scoop-upstream
  namespace: tools
spec:
  schedule: "0 3 * * *"
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: OnFailure
          containers:
            - name: scoop-upstream
              image: registry.example.com/scoop-upstream:latest
              args: ["-config", "/config/config.yaml"]
              volumeMounts:
                - name: config
                  mountPath: /config
                  readOnly: true
                - name: cache
                  mountPath: /var/cache/scoop-upstream
              env:
                - name: ARTIFACTORY_API_KEY
                  valueFrom:
                    secretKeyRef:
                      name: scoop-upstream-secrets
                      key: artifactory-api-key
                - name: GIT_AUTH_TOKEN
                  valueFrom:
                    secretKeyRef:
                      name: scoop-upstream-secrets
                      key: git-auth-token
          volumes:
            - name: config
              configMap:
                name: scoop-upstream-config
            - name: cache
              persistentVolumeClaim:
                claimName: scoop-upstream-cache
```

Mount the `config.yaml` as a `ConfigMap` and secrets as a `Secret`. The PVC for `/var/cache/scoop-upstream` keeps the Git clones between runs, avoiding a full re-clone each time.

---

## Exit codes

| Code | Meaning |
|---|---|
| `0` | All apps synced successfully (or nothing to do). |
| `1` | One or more apps failed to sync, or the Git push failed. |

Individual app failures are logged but do not abort the run — the rest of the allowlist continues. The non-zero exit code signals that attention is needed.
