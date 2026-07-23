FROM alpine
RUN apk add --no-cache bash krb5 krb5-server krb5-pkinit
EXPOSE 88/udp
