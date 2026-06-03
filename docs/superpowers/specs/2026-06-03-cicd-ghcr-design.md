# CI/CD: Docker Image Release to GHCR — Design

**Date:** 2026-06-03
**Status:** Approved (pending spec review)
**Branch:** `feat/cicd-ghcr`

## Goal

Add continuous integration and a release pipeline that publishes a multi-arch
Docker image of the tracker service to GitHub Container Registry (GHCR).

Two outcomes:

1. **CI** — every pull request and push to `main` runs `go build`, `go vet`,
   and `go test -race ./...` (the project currently has no automated test
   workflow, despite CLAUDE.md requiring the race detector).
2. **CD** — pushing a semver tag `vX.Y.Z` runs the same test suite and, on
   success, builds and pushes a multi-arch image to
   `ghcr.io/bikinitop/tracker` tagged `:X.Y.Z`, `:X.Y`, `:X`, and `:latest`.

## Non-Goals

- Deployment to any runtime (k8s, ECS, etc.). This produces and publishes an
  image; deploying it is out of scope.
- Rolling `:edge`/main builds. Only tag-driven versioned releases publish.
- Signing/SBOM/provenance attestation. Can be added later; not in this iteration.

## Decisions (settled during brainstorming)

| Topic | Decision |
|-------|----------|
| Release trigger | Git tag `v*` → versioned release |
| Test gating | CI on PR + push to main; release re-runs tests and is gated on them |
| Test reuse | A single reusable workflow (`workflow_call`) — one source of truth |
| Runtime base image | `gcr.io/distroless/static:nonroot` (multi-stage build) |
| Architectures | `linux/amd64` + `linux/arm64` (buildx + QEMU) |
| Version stamping | Inject the git tag into the binary via `-ldflags`, logged at startup |
| Image visibility | Public on GHCR |

## Architecture

Four new files plus one tiny code change, each with a single responsibility:

```
Dockerfile                       # multi-stage build → distroless static image
.dockerignore                    # keep build context small
.github/workflows/go-test.yml    # reusable: build + vet + race test
.github/workflows/ci.yml         # PR + push to main → calls go-test.yml
.github/workflows/release.yml    # tag v* → test (calls go-test.yml) + publish
cmd/tracker/main.go              # add `var version`, log it at startup
```

### Component 1: Dockerfile (multi-stage)

**Builder stage** (`golang:1.25`):
- Copy `go.mod`/`go.sum`, `go mod download` (cached layer).
- Copy source, build:
  `CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /tracker ./cmd/tracker`
- `VERSION` is a build-arg (defaults to `dev`). `TARGETOS`/`TARGETARCH` are
  provided automatically by buildx for cross-compilation — Go cross-compiles
  natively, so arm64 builds do **not** pay the QEMU emulation cost for the
  compile step (only the final image assembly differs per platform).

**Runtime stage** (`gcr.io/distroless/static:nonroot`):
- `COPY --from=builder /tracker /tracker`
- `EXPOSE 8080`
- `USER nonroot:nonroot` (distroless `nonroot` already defaults to this)
- `ENTRYPOINT ["/tracker"]`

Rationale: distroless `static:nonroot` ships CA certs + tzdata, runs as
non-root, has no shell/package manager (minimal attack surface). The static
binary needs no libc.

### Component 2: `.dockerignore`

Exclude from the build context: `.git`, `.github`, `docs`, `*_test.go` is not
excluded (they don't ship anyway since only `./cmd/tracker` is built, but the
context stays small by dropping `.git` and `docs`). Include only what the build
needs (Go sources, `go.mod`, `go.sum`).

### Component 3: `go-test.yml` (reusable)

```yaml
on:
  workflow_call:
```
Single job on `ubuntu-latest`:
1. `actions/checkout@v4`
2. `actions/setup-go@v5` with `go-version-file: go.mod`, `cache: true`
3. `go build ./...`
4. `go vet ./...`
5. `go test -race ./...`

This is the canonical definition of "the code passes." Both `ci.yml` and
`release.yml` invoke it so the two can never drift.

### Component 4: `ci.yml`

```yaml
on:
  pull_request:
  push:
    branches: [main]
permissions:
  contents: read
jobs:
  test:
    uses: ./.github/workflows/go-test.yml
```

Gives every PR (and main) the race-tested gate that is currently missing.

### Component 5: `release.yml`

```yaml
on:
  push:
    tags: ['v*']
```

- **Job `test`** — `uses: ./.github/workflows/go-test.yml`.
- **Job `publish`** — `needs: test`, `permissions: { contents: read, packages: write }`:
  1. `actions/checkout@v4`
  2. `docker/setup-qemu-action@v3` (enables arm64 emulation for image assembly)
  3. `docker/setup-buildx-action@v3`
  4. `docker/login-action@v3` → registry `ghcr.io`, user `${{ github.actor }}`,
     password `${{ secrets.GITHUB_TOKEN }}`
  5. `docker/metadata-action@v5` → images `ghcr.io/bikinitop/tracker`,
     tags from semver:
     `type=semver,pattern={{version}}`, `{{major}}.{{minor}}`, `{{major}}`,
     plus `type=raw,value=latest`. Emits OCI labels too.
  6. `docker/build-push-action@v6`:
     - `platforms: linux/amd64,linux/arm64`
     - `build-args: VERSION=${{ github.ref_name }}`
     - `tags`/`labels` from metadata-action
     - `push: true`
     - `cache-from: type=gha`, `cache-to: type=gha,mode=max`

The built-in `GITHUB_TOKEN` with `packages: write` authorizes pushing to the
repo's GHCR namespace; no PAT needed. Image name must be lowercase
(`bikinitop/tracker` already is).

### Component 6: version stamping (`cmd/tracker/main.go`)

Add a package-level `var version = "dev"` and log it once at startup
(e.g. in `run()` before `ListenAndServe`):
`log.Printf("tracker version %s starting on %s", version, srv.addr)`.

The release build injects the tag via `-ldflags "-X main.version=${VERSION}"`,
so a running container reports exactly which release it is. Local/`dev` builds
report `dev`.

## Testing & Verification

CI/CD config is verified by execution, not unit tests:

- **YAML validity / wiring:** push the branch; the `ci.yml` run must pass
  (build + vet + race test green) — proves the reusable workflow works.
- **Dockerfile builds locally:** `docker build --build-arg VERSION=test -t tracker:test .`
  succeeds and `docker run --rm -e NATS_URL=disabled -p 8080:8080 tracker:test`
  serves `/health` → `ok` and logs `tracker version test ...`.
- **Release path:** validated by a real tag (e.g. `v0.1.0`) after merge, or a
  dry-run via `workflow_dispatch` is explicitly out of scope. The release job's
  correctness is reviewed statically; first real tag is the live test. After the
  first publish, set the GHCR package visibility to **Public** once in the
  package settings (one-time manual step; see Risks).
- **Go change:** existing `go test -race ./...` must stay green (the `version`
  var + startup log don't alter behavior; `cmd/tracker` tests still pass).

## Risks / Trade-offs

- **arm64 image assembly uses QEMU**, which is slow — but Go cross-compiles the
  binary natively (via `TARGETARCH`), so only the thin final-stage `COPY` runs
  under emulation. Net build time is modest. GHA layer cache (`type=gha`)
  further reduces repeat builds.
- **`GITHUB_TOKEN` GHCR push** requires `packages: write` and that the package
  (first publish) is linked to the repo — automatic for `ghcr.io/<owner>/<repo>`.
- **distroless has no shell**, so `docker exec` debugging isn't possible; this
  is an accepted security trade-off (use `:debug` variant ad hoc if ever needed).
- **Public visibility is a one-time manual step.** GHCR publishes a package as
  private on first push; `GITHUB_TOKEN` cannot flip visibility. After the first
  release, set the package to **Public** once in GitHub → Packages → tracker →
  Package settings → Change visibility. Thereafter anonymous `docker pull`
  works with no pull secret. This manual step is documented as part of the
  release runbook (see Verification).

## Out-of-scope follow-ups (noted, not built)

- `:edge` builds on push to main.
- Cosign signing + SBOM + build provenance attestation.
- Deployment workflow.
