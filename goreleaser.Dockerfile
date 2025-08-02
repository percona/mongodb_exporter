# This is a custom goreleaser Dockerfile to help us prepare goreleaser images with kerberos libraries.
# That way, we can release mongodb_exporter binaries with support for GSSAPI authentication.
FROM golang:1.24-bookworm AS installer

RUN go install github.com/goreleaser/goreleaser/v2@latest

FROM golang:1.24-bookworm

RUN apt-get update && apt-get install -y \
    git \
    make \
    bash \
    curl \
    docker.io \
    libkrb5-dev \
    gcc \
    gcc-x86-64-linux-gnu \
    libc6-dev \
    libc6-dev-amd64-cross \
    ca-certificates \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

#RUN git config --global --add safe.directory '*'
#RUN git config --global url."https://".insteadOf git://

COPY --from=installer /go/bin/goreleaser /go/bin/goreleaser
ENTRYPOINT ["/go/bin/goreleaser"]