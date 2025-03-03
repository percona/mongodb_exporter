FROM alpine
RUN apk add --no-cache ca-certificates
USER 65535:65535
COPY ./mongodb_exporter /
EXPOSE 9216
