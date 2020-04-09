FROM golang:1.13

LABEL maintainer="Meik Minks <mminks@inoxio.de>"

WORKDIR /go/src/github.com/percona/mongodb_exporter

COPY . .

RUN make build

FROM quay.io/prometheus/busybox:latest

LABEL maintainer="Alexey Palazhchenko <alexey.palazhchenko@percona.com>"

COPY --from=0 /go/src/github.com/percona/mongodb_exporter/bin/mongodb_exporter /bin/mongodb_exporter

EXPOSE 9216

ENTRYPOINT [ "/bin/mongodb_exporter" ]

