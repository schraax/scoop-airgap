---
layout: default
title: Home
nav_order: 1
---

# scoop-airgap

**scoop-airgap** is a companion tool for [Scoop](https://scoop.sh) that enables fully airgapped Windows environments to install and update software packages without direct internet access.

It runs on a Linux host (or in a container) that *does* have internet access, mirrors selected packages to an internal [Artifactory](https://jfrog.com/artifactory/) instance, and publishes rewritten bucket definitions to an internal Git server. Airgapped Windows clients point Scoop at the internal buckets and download everything from Artifactory — the public internet is never reached from inside the secure network.

---

## How it works

```
  Internet                          Internal network (airgapped)
  ───────                           ────────────────────────────

  GitHub                            Artifactory
  ├─ ScoopInstaller/Main  ──┐       └─ scoop-mirror/main/git/2.43.0/Git-2.43.0-64-bit.exe
  └─ ScoopInstaller/Extras ─┤            ↑ download
                             │
                       scoop-airgap    Internal Git server
                             │       ├─ git.example.com/scoop/main.git
                             └──────►│     └─ bucket/git.json   (URLs → Artifactory)
                                     └─ git.example.com/scoop/extras.git
                                           └─ bucket/vscode.json

                                     Windows client (Scoop)
                                     └─ scoop bucket add internal-main …/main.git
                                     └─ scoop install git   →  downloads from Artifactory
```

1. **scoop-airgap** fetches manifest JSON files from public Scoop bucket repos.
2. For each listed app it downloads the installer binaries and uploads them to Artifactory.
3. It rewrites the manifest URLs to point at Artifactory and pushes the adapted manifests to internal Git repositories that have the same layout as standard Scoop buckets.
4. Airgapped Windows clients add those internal Git repos as Scoop buckets and install apps normally — Scoop cannot tell the difference.

---

## Quick links

| Audience | Start here |
|---|---|
| Setting up the mirror | [Operator Guide]({{ '/operator/' | relative_url }}) |
| Using Scoop on an airgapped machine | [User Guide]({{ '/user/' | relative_url }}) |
| Understanding the components | [Architecture]({{ '/architecture/' | relative_url }}) |
