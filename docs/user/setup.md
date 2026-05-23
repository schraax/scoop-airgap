---
layout: default
title: Initial Setup
parent: User Guide
nav_order: 1
---

# Initial Setup
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Prerequisites

- Windows 10 or later (PowerShell 5.1 is included; PowerShell 7 recommended).
- Access to the internal Git server where the mirrored bucket definitions are hosted. Ask your IT team for the bucket URLs.
- Network access to the internal Artifactory instance (usually available on the corporate network or VPN).

---

## 1. Install Scoop

If Scoop is not already installed, your IT team should provide an offline installer or a bootstrap script. A typical bootstrap (run in PowerShell as your normal user, **not** as Administrator) looks like:

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
Invoke-RestMethod -Uri https://get.scoop.sh | Invoke-Expression
```

In an airgapped environment the bootstrap script itself may need to come from an internal source. Follow your organisation's specific instructions for this step.

---

## 2. Disable the default buckets

Scoop adds a `main` bucket pointing to GitHub by default. Remove it so that Scoop does not attempt to reach the internet:

```powershell
scoop bucket rm main
```

If you see an error that the bucket does not exist, that is fine — continue to the next step.

---

## 3. Add the internal buckets

Your IT team will give you one URL per bucket. Add each one with `scoop bucket add`:

```powershell
scoop bucket add internal-main  https://git.example.com/scoop/main.git
scoop bucket add internal-extras https://git.example.com/scoop/extras.git
```

Replace the URLs with the ones provided by your IT team. The names (`internal-main`, `internal-extras`) are local aliases you can choose freely — they do not need to match the upstream bucket names.

To confirm the buckets were added:

```powershell
scoop bucket list
```

You should see your internal buckets listed. Scoop clones each bucket repo locally during `bucket add`.

---

## 4. (Optional) Prevent Scoop from checking for updates to itself

By default Scoop checks for its own updates on the internet. Disable this in an airgapped environment:

```powershell
scoop config scoop_repo ""
```

Or configure Scoop to point at an internal mirror of the Scoop repository:

```powershell
scoop config scoop_repo https://git.example.com/scoop/scoop.git
```

Ask your IT team whether an internal Scoop mirror has been set up.

---

## 5. Verify the setup

Install a small package to confirm everything is working end-to-end:

```powershell
scoop install jq
jq --version
```

If the install succeeds and `jq` reports its version, the mirror is configured correctly.

---

## Proxy settings

If your organisation routes traffic through an HTTP proxy, set the standard Windows proxy environment variables before running Scoop commands:

```powershell
$env:HTTP_PROXY  = "http://proxy.example.com:8080"
$env:HTTPS_PROXY = "http://proxy.example.com:8080"
$env:NO_PROXY    = "git.example.com,artifactory.example.com"
```

Add these to your PowerShell profile (`$PROFILE`) to make them permanent.
