FROM alpine:3.23.4@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11 AS builder
RUN apk add --no-cache ca-certificates

FROM scratch AS final
USER 65535:65535
COPY  --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY ./mongodb_exporter /
EXPOSE 9216
ENTRYPOINT ["/mongodb_exporter"]
