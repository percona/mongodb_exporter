FROM       alpine:latest
MAINTAINER David Cuadrado <dacuad@facebook.com>
EXPOSE     9001

ENV  GOPATH /go
ENV APPPATH $GOPATH/src/github.com/dcu/mongodb_exporter
COPY . $APPPATH
RUN apk add --update -t build-deps go git mercurial libc-dev gcc libgcc \
    && cd $APPPATH && go get -d && go build -o /bin/mongodb_exporter \
    && apk del --purge build-deps && rm -rf $GOPATH

ENTRYPOINT [ "/bin/mongodb_exporter" ]
