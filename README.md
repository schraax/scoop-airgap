# scoop-airgap

A companion tool for [Scoop](https://scoop.sh) that lets fully airgapped Windows environments install and update packages without ever reaching the public internet.

It runs on a Linux host (or container) with internet access, mirrors selected app binaries to an internal storage server (Artifactory, Nexus, nginx WebDAV, …), and publishes rewritten bucket manifests to an internal Git server. Airgapped Windows clients add those Git repos as normal Scoop buckets — they cannot tell the difference.

```
  Internet                       Internal network (airgapped)
  ──────────────────             ────────────────────────────
  GitHub (Scoop buckets)         Artifactory / Nexus / WebDAV
  └─ ScoopInstaller/Main ──┐     └─ scoop-mirror/main/git/2.43.0/Git-2.43.0-64-bit.exe
                            │
                      scoop-airgap    Internal Git server
                            │     ├─ git.example.com/scoop/main.git
                            └────►│     └─ bucket/git.json  (URLs → internal)
                                  └─ git.example.com/scoop/extras.git

                                  Windows client
                                  └─ scoop bucket add main https://git.example.com/scoop/main.git
                                  └─ scoop install git   ← downloads from Artifactory
```

## Documentation

Full documentation is published at **https://schraax.github.io/scoop-airgap/**.

| Guide | Description |
|---|---|
| [Operator Guide](https://schraax.github.io/scoop-airgap/operator/) | Install, configure, and run scoop-airgap |
| [User Guide](https://schraax.github.io/scoop-airgap/user/) | Set up Scoop on an airgapped Windows machine |
| [Architecture](https://schraax.github.io/scoop-airgap/architecture) | Component overview and data flow |

## Quick start

```bash
# Build
CGO_ENABLED=0 go build -o scoop-airgap ./cmd/scoop-airgap

# Or run in Docker
docker run --rm \
  -v /etc/scoop-airgap:/config:ro \
  ghcr.io/schraax/scoop-airgap
```

See the [Operator Guide](https://schraax.github.io/scoop-airgap/operator/) for a full configuration reference.

## License

MIT
