# syntax=docker/dockerfile:1

# Build stage: compile the API server. The module lives in backend/ (module path
# tigerswap/backend), so the build context is backend/.
FROM golang:1.26-bookworm AS build

WORKDIR /src

# Resolve dependencies first so a source-only change does not re-download them.
# go.sum is not fully vendored, hence -mod=mod.
ENV GOFLAGS=-mod=mod
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# The default build uses the stub bot bridge (services/botengine/bridge_stub.go),
# so the server links no cgo. To enable the Rust strategy engine, build
# libtigerswap_bots.so from backend/rust and build with `-tags rust` plus
# CGO_ENABLED=1 and LD_LIBRARY_PATH pointing at the .so; see AGENTS.md.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/tigerswap-api ./cmd/api

# Runtime stage: a minimal image with only the server and its CA bundle.
FROM debian:bookworm-slim

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates \
 && rm -rf /var/lib/apt/lists/*

# Run unprivileged.
RUN useradd --system --uid 10001 --home /app tigerswap
WORKDIR /app

COPY --from=build /out/tigerswap-api /app/tigerswap-api

USER tigerswap
EXPOSE 8080

# The server reads PORT (default 8080) and connects to Postgres via the DB_*
# variables; there are no defaults for the database, by design.
ENTRYPOINT ["/app/tigerswap-api"]
