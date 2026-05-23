---
layout: default
title: Architecture
nav_order: 2
permalink: /architecture/
---

# Architecture
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Components

### scoop-airgap (this tool)

A single statically-linked Go binary. It is the only component that needs internet access. It is intended to run periodically — for example as a container in a cron job or a CI pipeline — on a host that bridges the public internet and the internal network.

### Artifactory (binary storage)

A generic Artifactory repository stores the installer binaries. Files are organised at the path:

```
{repo}/{bucket}/{app}/{version}/{filename}
```

Example:

```
scoop-mirror/main/git/2.43.0/Git-2.43.0-64-bit.exe
scoop-mirror/extras/vscode/1.88.0/VSCodeSetup-x64-1.88.0.exe
```

Because scoop-airgap checks for existence before uploading, reruns are cheap — only new versions are transferred.

### Internal Git server (bucket hosting)

Each mirrored upstream bucket gets a corresponding internal Git repository. The repository layout is identical to a standard Scoop bucket:

```
bucket/
├── git.json
├── curl.json
└── jq.json
```

This means airgapped clients can add them with the standard `scoop bucket add` command. Only the `url` fields inside the JSON manifests differ — they point to Artifactory instead of GitHub or vendor CDNs. Hash values are unchanged because the files are byte-identical.

### Windows clients (Scoop)

Standard [Scoop](https://scoop.sh) — no modifications required. Clients are configured to use only internal bucket repos and have no route to the internet.

---

## Data flow

```
┌─────────────────────────────────────────────────────────────────┐
│                        scoop-airgap                           │
│                                                                 │
│  1. Read config.yaml (allowlist of buckets + apps)              │
│                                                                 │
│  2. For each app:                                               │
│     a. GET {public_bucket_url}/{app}.json                       │
│     b. Parse all url fields (top-level + per-architecture)      │
│     c. HEAD {artifactory}/{bucket}/{app}/{ver}/{file}           │
│        → skip if already present                                │
│     d. GET {original_download_url}                              │
│        → verify SHA-256 / SHA-512 hash                          │
│     e. PUT {artifactory}/{bucket}/{app}/{ver}/{file}            │
│     f. Rewrite url fields in manifest JSON → Artifactory URLs   │
│                                                                 │
│  3. git pull  internal bucket repo                              │
│  4. Write bucket/{app}.json (rewritten manifest)                │
│  5. git commit && git push                                      │
└─────────────────────────────────────────────────────────────────┘
```

Steps 2a–2f run concurrently across a configurable worker pool (default: 4). The Git push happens once per bucket after all apps in that bucket are processed.

---

## Manifest rewriting

Only the `url` fields are modified. Everything else — hashes, install scripts, `bin`, `shortcuts`, `autoupdate` metadata — is preserved verbatim.

**Original manifest (`bucket/git.json` on GitHub):**

```json
{
    "version": "2.43.0",
    "url": "https://github.com/git-for-windows/git/releases/download/v2.43.0.windows.1/Git-2.43.0-64-bit.exe#/git-2.43.0-64-bit.exe",
    "hash": "sha256:3a3e23a9a09d6fb5aefc6c5aa0a0aec0f51cff22a74f2dab59cf7ee3d6428bec"
}
```

**Rewritten manifest (pushed to internal Git repo):**

```json
{
    "version": "2.43.0",
    "url": "https://artifactory.example.com/artifactory/scoop-mirror/main/git/2.43.0/Git-2.43.0-64-bit.exe#/git-2.43.0-64-bit.exe",
    "hash": "sha256:3a3e23a9a09d6fb5aefc6c5aa0a0aec0f51cff22a74f2dab59cf7ee3d6428bec"
}
```

The Scoop rename fragment (`#/git-2.43.0-64-bit.exe`) is preserved so Scoop's download behaviour is unchanged. The hash remains identical because the binary is not modified.

Manifests with per-architecture URLs (32-bit / 64-bit / arm64) are handled the same way — each architecture block is rewritten independently.

---

## Security considerations

- Hashes are verified after download and before upload. A corrupted or tampered binary is rejected before it reaches Artifactory.
- Artifactory credentials and Git tokens are never stored in the config file; they are referenced as environment variables (`${VAR}`).
- The tool only writes to Artifactory and the internal Git server. It makes no outbound connections from the airgapped network; the binary itself runs outside the airgap.
- The `autoupdate` and `checkver` sections in manifests still reference public URLs. They are harmless on airgapped clients (Scoop's `scoop update` reads new manifests from the internal Git repo, not from `checkver`), but they can be stripped in a future version if desired.
