FROM registry.access.redhat.com/ubi9/ubi-minimal AS certs

FROM registry.access.redhat.com/ubi9/ubi-micro
ARG TARGETPLATFORM
# GoReleaser stages the binaries under $TARGETPLATFORM in its build context;
# `make docker-build` overrides BIN_DIR with the local build path instead.
ARG BIN_DIR=$TARGETPLATFORM

LABEL name="Percona MongoDB Exporter" \
      vendor="Percona" \
      summary="Prometheus exporter for MongoDB" \
      description="Prometheus exporter for MongoDB server, replica set and sharded cluster metrics" \
      maintainer="Percona Development <info@percona.com>"

# ubi-micro ships no CA bundle; take the one from ubi-minimal.
COPY --from=certs /etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem /etc/pki/tls/certs/ca-bundle.crt
COPY LICENSE /licenses/
COPY $BIN_DIR/mongodb_exporter /
USER 65535:65535
EXPOSE 9216
ENTRYPOINT ["/mongodb_exporter"]
