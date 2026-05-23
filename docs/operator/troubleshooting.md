---
layout: default
title: Troubleshooting
parent: Operator Guide
nav_order: 4
---

# Operator Troubleshooting
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Manifest not found

```
ERROR main/myapp: manifest not found: https://raw.githubusercontent.com/…/bucket/myapp.json
```

**Cause:** The app name does not exist in the upstream bucket, or the bucket URL is wrong.

**Fix:**
- Verify the app name by browsing the upstream bucket on GitHub (e.g. `https://github.com/ScoopInstaller/Main/tree/master/bucket`).
- Check that `buckets[].url` points to the raw base URL, not the HTML page.
- Confirm the correct branch name (`master` vs `main`) in the URL.

---

## Hash mismatch

```
ERROR main/git: download https://…/Git-2.43.0-64-bit.exe: hash mismatch (sha256): got abc…, want def…
```

**Cause:** The downloaded file does not match the hash in the manifest. This can be caused by:
- A CDN serving a corrupted file (transient).
- The upstream manifest was updated between when scoop-airgap fetched it and when the binary was downloaded.

**Fix:**
- Rerun scoop-airgap. If the error persists, the upstream manifest itself may be incorrect — check the Scoop bucket issue tracker.
- Use `-force` to re-attempt the download on the next run.

---

## Artifactory upload fails (403 Forbidden)

```
ERROR main/git: upload main/git/2.43.0/Git-2.43.0-64-bit.exe: PUT …: HTTP 403
```

**Cause:** The Artifactory credentials do not have deploy permission on the target repository.

**Fix:**
- Verify `ARTIFACTORY_API_KEY` is set correctly.
- In Artifactory, confirm the service account has **Deploy/Cache** permission on the `scoop-mirror` repository.
- If using Basic auth (`username` + `api_key`), ensure the username matches the account that owns the API key.

---

## Git push fails

```
ERROR git push for bucket main: git push origin main: exit status 1
remote: error: permission denied
```

**Cause:** The Git token does not have write access to the internal bucket repo.

**Fix:**
- Verify `GIT_AUTH_TOKEN` is set and not expired.
- Confirm the token has at least **Write** (or **Maintainer**) access to the repo.
- For SSH auth, ensure the SSH key is loaded (`ssh-add`) and the host key is in `known_hosts`.

---

## Git push fails — non-fast-forward

```
ERROR git push for bucket main: git push origin main: exit status 1
hint: Updates were rejected because the tip of your current branch is behind its remote counterpart.
```

**Cause:** The remote branch has commits that the local clone does not have. This can happen if the repo was pushed to by another process or if the local clone is stale.

**Fix:**
- Delete the local clone directory (under `git.local_path_base/{bucket}`) and rerun. scoop-airgap will do a fresh clone.
- Investigate who else has push access and whether parallel runs are occurring.

---

## App syncs but manifest has wrong URLs

**Symptom:** After a sync, the JSON in the internal Git repo still contains the original GitHub/CDN URLs.

**Cause:** The manifest was written but the URL rewriting did not match those fields. This can happen with unusual manifest structures.

**Fix:**
- Run with a specific app: `scoop-airgap -config config.yaml -app myapp -dry-run` and review the log output.
- Check the manifest JSON in the upstream bucket to see if it uses a non-standard structure (e.g. nested `installer.url` fields — these are not currently rewritten, see [Architecture](../architecture.md)).

---

## Local clone accumulates disk space

**Cause:** If `git.local_path_base` is set, the clones persist between runs and grow with each commit (Git object history).

**Fix:** This is normal and generally desirable (avoids re-cloning). To reclaim space, run `git gc` inside the clone directories, or periodically delete and re-clone:

```bash
rm -rf /var/cache/scoop-airgap/main
# scoop-airgap will re-clone on the next run
```

---

## All apps fail with "dial tcp: no such host"

**Cause:** The host running scoop-airgap cannot reach the public internet.

**Fix:** scoop-airgap must run on a host with internet access. If you are containerising it, ensure the container has an external network interface and DNS resolution works (`curl https://raw.githubusercontent.com` from inside the container should succeed).
