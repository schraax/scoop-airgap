# syntax=docker/dockerfile:1

# ── Build ─────────────────────────────────────────────────────────────────────
FROM cgr.dev/chainguard/go:latest AS build

WORKDIR /src

# Fetch dependencies in a separate layer so it is cached across code changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 produces a fully static binary; -s -w strips debug info;
# -trimpath removes local build paths from the binary.
RUN CGO_ENABLED=0 GOOS=linux go build \
      -ldflags="-s -w" \
      -trimpath \
      -o /scoop-upstream \
      ./cmd/scoop-upstream

# ── Runtime ───────────────────────────────────────────────────────────────────
# cgr.dev/chainguard/git is a distroless-style image: no shell, no package
# manager, only git and CA certificates. Runs as non-root (uid 65532) by default.
FROM cgr.dev/chainguard/git:latest

COPY --from=build /scoop-upstream /usr/local/bin/scoop-upstream

# /config  — mount a read-only ConfigMap or bind-mount containing config.yaml
# /cache   — persistent volume for git clones; avoids re-cloning on every run
VOLUME ["/config", "/cache"]

ENTRYPOINT ["scoop-upstream"]
CMD ["-config", "/config/config.yaml"]
