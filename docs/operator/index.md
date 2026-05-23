---
layout: default
title: Operator Guide
nav_order: 3
has_children: true
---

# Operator Guide

This guide is for the team responsible for running **scoop-airgap** and maintaining the internal package mirror.

## What you will set up

| Component | Your responsibility |
|---|---|
| **scoop-airgap** binary | Build or pull the container image; schedule it to run periodically |
| **Artifactory** generic repo | Create the repo; supply credentials to scoop-airgap |
| **Internal Git repos** | Create one repo per mirrored bucket; supply access token to scoop-airgap |
| **config.yaml** | Maintain the allowlist of buckets and apps |

## Guides in this section

- [Installation](installation.md) — building from source or using Docker
- [Configuration](configuration.md) — full `config.yaml` reference
- [Running](running.md) — one-shot, scheduled, and container operation
- [Troubleshooting](troubleshooting.md) — diagnosing common problems
