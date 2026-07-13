FROM alpine:3.20
RUN apk add --no-cache bash krb5 krb5-server krb5-pkinit
EXPOSE 88/udp
