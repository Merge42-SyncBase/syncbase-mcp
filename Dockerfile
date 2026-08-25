# syntax=docker/dockerfile:1.7
ARG GO_IMAGE=golang:1.26.6-bookworm@sha256:116d58cbd88c1297624acc6e967a060012422bacf9930927e23fb719189c6f36
ARG RUNTIME_IMAGE=debian:bookworm-slim@sha256:abd67ffcfa541b485a3dff59865ab629aa048a6c613e639d36e7456b0b229241

FROM ${GO_IMAGE} AS build
ARG BINARY=syncbase
WORKDIR /src

COPY go.mod go.sum ./
# Round-1 first-party modules are public and checksum-verifiable. Anonymous
# builds must not require a publication credential.
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download all

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -trimpath -ldflags='-s -w' -o "/out/${BINARY}" ./cmd/mcp

FROM ${RUNTIME_IMAGE}
ARG BINARY=syncbase
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates libgomp1 \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 10001 syncbase \
    && useradd --uid 10001 --gid 10001 --no-create-home --shell /usr/sbin/nologin syncbase \
    && mkdir -p /app \
    && chown -R syncbase:syncbase /app
COPY --from=build "/out/${BINARY}" /app/syncbase
COPY LICENSE THIRD_PARTY_NOTICES.md /usr/share/licenses/syncbase-mcp/
USER 10001:10001
WORKDIR /app
ENTRYPOINT ["/app/syncbase"]
