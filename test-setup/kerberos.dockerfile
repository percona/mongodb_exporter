FROM debian:bookworm-slim
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      bash \
      krb5-kdc \
      krb5-admin-server \
      krb5-user; \
    rm -rf /var/lib/apt/lists/*
EXPOSE 88/udp
