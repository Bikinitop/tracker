# Releasing

The tracker is released as a multi-arch Docker image to GitHub Container
Registry (GHCR) at `ghcr.io/bikinitop/tracker`.

## Cutting a release

1. Ensure `main` is green (CI passes).
2. Tag a semver version on `main` and push the tag:
   ```bash
   git checkout main && git pull
   git tag v1.0.0
   git push origin v1.0.0
   ```
3. The `Release` workflow runs the test suite, then builds and pushes:
   - `ghcr.io/bikinitop/tracker:1.0.0`
   - `ghcr.io/bikinitop/tracker:1.0`
   - `ghcr.io/bikinitop/tracker:1`
   - `ghcr.io/bikinitop/tracker:latest`

   for `linux/amd64` and `linux/arm64`.

## Make the package public (one-time, manual)

GHCR publishes a new package as **private**, and `GITHUB_TOKEN` cannot change
visibility. After the first release:

1. Go to GitHub → the `bikinitop` org/user → **Packages** → `tracker`.
2. **Package settings** → **Danger Zone** → **Change visibility** → **Public**.

Thereafter anyone can `docker pull ghcr.io/bikinitop/tracker:<tag>` without auth.

## Running the image

```bash
docker run --rm -p 8080:8080 \
  -e NATS_URL=nats://your-nats:4222 \
  ghcr.io/bikinitop/tracker:latest
```

The container listens on `8080` by default; override with `-e TRACKER_PORT=...`.

On startup it logs a line ending in `tracker version v1.0.0 starting on :8080`
(prefixed with the standard Go `log` timestamp).
