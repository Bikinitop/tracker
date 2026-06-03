# syntax=docker/dockerfile:1

# ---- Builder ----
FROM --platform=$BUILDPLATFORM golang:1.25 AS builder

# buildx provides these automatically; declare them to use in the build.
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /src

# Cached dependency layer.
COPY go.mod go.sum ./
RUN go mod download

# Build the static binary. Go cross-compiles natively via GOOS/GOARCH, so the
# arm64 build does not need QEMU emulation for the compile step.
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
    -o /tracker ./cmd/tracker

# ---- Runtime ----
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /tracker /tracker

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/tracker"]
