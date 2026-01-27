# Multi-stage build: Builder stage
FROM golang:1.25-trixie AS builder

ARG TARGETPLATFORM
WORKDIR /app

# Install Node.js for frontend build
RUN apt-get update && apt-get install -y npm && \
    npm install -g n@latest && \
    n 22.19.0 && \
    rm -rf /var/lib/apt/lists/*

# Download and install build tools for correct architecture
RUN mkdir -p /tmp/downloads && cd /tmp/downloads && \
    if [ "${TARGETPLATFORM}" = "linux/arm64" ]; then \
        echo "Downloading arm64 binaries" && \
        wget --no-verbose https://github.com/go-task/task/releases/download/v3.45.4/task_linux_arm64.tar.gz && \
        tar -xzf task_linux_arm64.tar.gz && mv ./task /usr/local/bin/task && \
        wget --no-verbose https://github.com/pressly/goose/releases/download/v3.25.0/goose_linux_arm64 && mv ./goose_linux_arm64 /usr/local/bin/goose && \
        wget --no-verbose https://github.com/sqlc-dev/sqlc/releases/download/v1.30.0/sqlc_1.30.0_linux_arm64.tar.gz && tar -xzf sqlc_1.30.0_linux_arm64.tar.gz && mv ./sqlc /usr/local/bin/sqlc && \
        wget --no-verbose https://github.com/golangci/golangci-lint/releases/download/v2.5.0/golangci-lint-2.5.0-linux-arm64.tar.gz && tar -xzf golangci-lint-2.5.0-linux-arm64.tar.gz && mv ./golangci-lint-2.5.0-linux-arm64/golangci-lint /usr/local/bin/golangci-lint; \
    else \
        echo "Downloading amd64 binaries" && \
        wget --no-verbose https://github.com/go-task/task/releases/download/v3.45.4/task_linux_amd64.tar.gz && tar -xzf task_linux_amd64.tar.gz && mv ./task /usr/local/bin/task && \
        wget --no-verbose https://github.com/pressly/goose/releases/download/v3.25.0/goose_linux_x86_64 && mv ./goose_linux_x86_64 /usr/local/bin/goose && \
        wget --no-verbose https://github.com/sqlc-dev/sqlc/releases/download/v1.30.0/sqlc_1.30.0_linux_amd64.tar.gz && tar -xzf sqlc_1.30.0_linux_amd64.tar.gz && mv ./sqlc /usr/local/bin/sqlc && \
        wget --no-verbose https://github.com/golangci/golangci-lint/releases/download/v2.5.0/golangci-lint-2.5.0-linux-amd64.tar.gz && tar -xzf golangci-lint-2.5.0-linux-amd64.tar.gz && mv ./golangci-lint-2.5.0-linux-amd64/golangci-lint /usr/local/bin/golangci-lint; \
    fi && chmod +x /usr/local/bin/* && rm -rf /tmp/downloads

RUN git config --global --add safe.directory '*'

COPY package.json package-lock.json ./
RUN npm install

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN task fixperms && task build

# Multi-stage build: Runtime stage
FROM debian:trixie-slim

LABEL org.opencontainers.image.source="https://github.com/eduardolat/pgbackweb"

WORKDIR /app
ENV DEBIAN_FRONTEND="noninteractive"

# Install runtime dependencies including curl and unzip
RUN set -e && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        curl unzip \
        postgresql-common ca-certificates tzdata wget unzip && \
    /usr/share/postgresql-common/pgdg/apt.postgresql.org.sh -y && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        postgresql-client-13 postgresql-client-14 \
        postgresql-client-15 postgresql-client-16 \
        postgresql-client-17 postgresql-client-18 && \
    apt-get clean autoclean && \
    apt-get autoremove --yes && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Create necessary directories
RUN mkdir -p /backups && chmod 777 /backups

# Copy goose from builder stage
COPY --from=builder /usr/local/bin/goose /usr/local/bin/goose

# Copy built binaries from builder stage
COPY --from=builder /app/dist/app /usr/local/bin/app
COPY --from=builder /app/dist/change-password /usr/local/bin/change-password

# Copy migrations, static files and templates
COPY --from=builder /app/internal/database/migrations ./internal/database/migrations
COPY --from=builder /app/internal/view/static ./internal/view/static

# Create non-root user
RUN groupadd -r pgbackweb && useradd -r -g pgbackweb -d /app -s /bin/bash pgbackweb && \
    chown -R pgbackweb:pgbackweb /app

USER pgbackweb

EXPOSE 8085

# Entrypoint script to handle migrations
RUN echo '#!/bin/bash\n\
set -e\n\
\n\
# Run migrations\n\
echo "Running database migrations..."\n\
goose -dir ./internal/database/migrations postgres "${PBW_POSTGRES_CONN_STRING}" up\n\
echo "Migrations completed"\n\
\n\
# Start the application\n\
exec /usr/local/bin/app' > /app/docker-entrypoint.sh && \
    chmod +x /app/docker-entrypoint.sh

ENTRYPOINT ["/app/docker-entrypoint.sh"]
