# syntax=docker/dockerfile:1.7

# ─── Stage 1: build the Go binary ──────────────────────────────────────────
FROM golang:1.25-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO is disabled because PocketBase uses modernc.org/sqlite (pure Go), so we
# can produce a fully static binary that runs on a slim base image.
# BUILD_TIME / BUILD_COMMIT are stamped into the binary so the running app can
# expose them (see internal/app build vars). GitHub Actions passes real values;
# local builds fall back to "unknown".
ARG BUILD_TIME=unknown
ARG BUILD_COMMIT=unknown
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X 'github.com/niclaswue/template-qr-photo/internal/app.BuildTime=${BUILD_TIME}' -X 'github.com/niclaswue/template-qr-photo/internal/app.BuildCommit=${BUILD_COMMIT}'" \
    -o /out/app ./cmd/app

# ─── Stage 2: fetch the Typst CLI ──────────────────────────────────────────
FROM debian:bookworm-slim AS typst

ARG TYPST_VERSION=v0.14.2
ARG TARGETARCH

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl xz-utils \
 && rm -rf /var/lib/apt/lists/*

RUN set -eux; \
    case "${TARGETARCH:-amd64}" in \
        amd64) arch="x86_64-unknown-linux-musl" ;; \
        arm64) arch="aarch64-unknown-linux-musl" ;; \
        *) echo "unsupported TARGETARCH: ${TARGETARCH}"; exit 1 ;; \
    esac; \
    curl -fsSL "https://github.com/typst/typst/releases/download/${TYPST_VERSION}/typst-${arch}.tar.xz" -o /tmp/typst.tar.xz; \
    mkdir -p /opt/typst; \
    tar -xJf /tmp/typst.tar.xz -C /opt/typst --strip-components=1; \
    /opt/typst/typst --version

# ─── Stage 3: minimal runtime ──────────────────────────────────────────────
FROM debian:bookworm-slim AS runtime

# wget is used by the container healthcheck (see deploy/docker-compose.yml) to
# hit /api/health; without it the probe can't run and the container is reported
# unhealthy even while serving fine.
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates tzdata wget \
 && rm -rf /var/lib/apt/lists/* \
 && useradd --system --create-home --uid 10001 --shell /usr/sbin/nologin app

WORKDIR /app

COPY --from=builder /out/app           /app/app
COPY --from=typst   /opt/typst/typst   /usr/local/bin/typst

# Static assets the running binary expects to find relative to its working
# directory. Mount your own config.json over /app/config.json to override.
COPY views        /app/views
COPY templates    /app/templates
COPY data         /app/data
COPY pb_public    /app/pb_public
COPY config.json  /app/config.json

RUN mkdir -p /app/pb_data && chown -R app:app /app

USER app

VOLUME ["/app/pb_data"]
EXPOSE 8090

ENTRYPOINT ["/app/app"]
CMD ["serve", "--http=0.0.0.0:8090"]
