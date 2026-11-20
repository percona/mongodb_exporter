FROM alpine AS builder
RUN apk add --no-cache ca-certificates

FROM scratch AS final
ARG TARGETPLATFORM
# GoReleaser stages the binaries under $TARGETPLATFORM in its build context;
# `make docker-build` overrides BIN_DIR with the local build path instead.
ARG BIN_DIR=$TARGETPLATFORM
USER 65535:65535
COPY  --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY $BIN_DIR/mongodb_exporter /
EXPOSE 9216
ENTRYPOINT ["/mongodb_exporter"]