---
layout: default
title: Troubleshooting
parent: User Guide
nav_order: 3
---

# User Troubleshooting
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## "App not found" when installing

```
ERROR 'myapp' isn't in any of your buckets. Try updating your buckets first by running 'scoop update'.
```

**Possible causes and fixes:**

1. **The bucket cache is stale.** Run `scoop update` to pull the latest manifest list from the internal Git repos, then retry.

2. **The app is not in the mirror allowlist.** Only apps explicitly listed by your IT team are mirrored. Contact your IT team and ask them to add the app to the allowlist and run a sync.

3. **The bucket is not added.** Run `scoop bucket list` to confirm your internal buckets are present. If they are missing, follow the [Initial Setup](setup.md) guide.

---

## Download fails with a connection error

```
ERROR https://artifactory.example.com/… : connection refused
```

**Possible causes and fixes:**

1. **VPN / network.** Ensure you are connected to the corporate network or VPN that can reach the Artifactory server.

2. **Proxy not configured.** If your organisation uses an HTTP proxy, set `$env:HTTPS_PROXY` as described in [Initial Setup](setup.md).

3. **Artifactory is down.** Contact your IT team to check the service status.

---

## Hash verification fails

```
ERROR hash check failed
```

**Cause:** The file Scoop downloaded does not match the hash in the manifest. This usually means the file in Artifactory was corrupted, or there is a network issue causing a partial download.

**Fix:**
- Clear Scoop's download cache and retry: `scoop cache rm myapp && scoop install myapp`.
- If it persists, contact your IT team — the artifact in Artifactory may need to be re-uploaded.

---

## `scoop update` fails with a Git error

```
Updating 'internal-main' bucket...
fatal: unable to access 'https://git.example.com/scoop/main.git/': Could not resolve host: git.example.com
```

**Cause:** Scoop cannot reach the internal Git server to pull the latest manifests.

**Fix:**
- Check VPN / network connectivity.
- Run `git ls-remote https://git.example.com/scoop/main.git` to test Git access directly.
- If your Git server requires authentication, ask your IT team whether a credential helper or token needs to be configured on your machine.

---

## An installed version is outdated and `scoop update` shows nothing new

**Cause:** The internal mirror has not yet been synced with the latest upstream version.

**Fix:** Contact your IT team and ask them to run a scoop-airgap sync to pick up the new version.

---

## `scoop bucket add` asks for a password

If Git prompts for credentials when adding a bucket, the bucket repo requires authentication that Scoop does not pass automatically.

**Fix:** Ask your IT team for the correct authentication method. Options include:
- A personal access token stored with the Windows Credential Manager.
- A read-only deploy token embedded in the bucket URL (your IT team can provide a pre-formed URL).
- SSH key authentication (if the URL uses `git@…` syntax).

---

## Scoop tries to reach the internet

**Symptom:** DNS lookups or connection attempts to `github.com` or other external hosts appear in network logs.

**Possible causes:**

1. The default `main` bucket was not removed. Run `scoop bucket list` — if `main` appears with a `https://github.com` URL, remove it: `scoop bucket rm main`.

2. Scoop is checking for its own updates. Disable this with `scoop config scoop_repo ""` or point it to an internal mirror.

3. A package's `checkver` or `autoupdate` metadata is being evaluated. This is a Scoop developer feature and should not trigger during normal `install` or `update` operations by end users. If it does, contact your IT team.
