---
layout: default
title: Daily Use
parent: User Guide
nav_order: 2
---

# Daily Use
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

Once the internal buckets are configured, you use Scoop exactly as you would in a standard (internet-connected) setup. This page is a quick reference for the most common operations.

---

## Searching for available packages

```powershell
scoop search git
```

This searches all locally cached bucket manifests. No internet connection is needed.

To list every package available in your internal buckets:

```powershell
scoop search ""
```

---

## Installing a package

```powershell
scoop install git
```

Scoop resolves the package from the internal bucket manifest and downloads the installer binary from Artifactory. The download URL in the manifest already points to the internal Artifactory instance — no internet access is required.

To install a specific version (if the `versions` bucket has been mirrored):

```powershell
scoop install versions/git19
```

---

## Updating packages

### Update bucket definitions

First, pull the latest manifest versions from the internal Git repos:

```powershell
scoop update
```

This runs `git pull` on each local bucket clone. It contacts only the internal Git server.

### Update installed packages

```powershell
scoop update *
```

Updates all installed packages to the versions currently in the internal buckets. To update a single package:

```powershell
scoop update git
```

{: .note }
New versions appear in the internal mirror only after your IT team has run a sync. If a version you need is not yet available, contact your IT team to request a mirror update.

---

## Listing installed packages

```powershell
scoop list
```

---

## Uninstalling a package

```powershell
scoop uninstall git
```

To remove old versions that are no longer the current one:

```powershell
scoop cleanup git
```

---

## Checking package status

```powershell
scoop status
```

Shows which installed packages have newer versions available in the mirrored buckets.

---

## Installing global packages (requires Administrator)

Some tools (those that modify system-wide `PATH` or install system services) need to be installed globally:

```powershell
# Run PowerShell as Administrator
scoop install --global 7zip
```

---

## Finding which bucket provides a package

```powershell
scoop search curl
```

The output shows the bucket name alongside each result. If a package you need is not listed, it may not be in the mirror allowlist yet — ask your IT team to add it.

---

## Common workflows

### Set up a new developer machine

```powershell
# Add internal buckets (one-time setup — see Initial Setup)
scoop bucket add internal-main https://git.example.com/scoop/main.git

# Install your standard toolset
scoop install git curl jq 7zip vscode
```

### Check for and apply all updates

```powershell
scoop update && scoop update *
```

### Install a package and verify it works

```powershell
scoop install jq
jq --version   # should print the version number
```
