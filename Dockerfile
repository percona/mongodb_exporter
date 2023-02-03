FROM golang:1.19-alpine

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

ADD exporter ./exporter
ADD internal ./internal
COPY *.go ./

RUN go build -o /mongodb_exporter

ENTRYPOINT [ "/mongodb_exporter" ]
