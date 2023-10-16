FROM alpine AS builder
RUN apk add --no-cache ca-certificates

FROM golang:alpine as builder2

RUN apk update && apk add make
RUN mkdir /source
COPY . /source
WORKDIR /source
RUN make init
RUN make build

FROM alpine AS final
USER 65535:65535
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder2 /source/mongodb_exporter /
EXPOSE 9216
ENTRYPOINT ["/mongodb_exporter"]