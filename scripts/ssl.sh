#!/bin/sh

set -x

cd $PWD/testdata

# CA
openssl genrsa -out ca.key 2048
openssl req -x509 -new -key ca.key -days 3650 -out ca.crt -subj "/CN=Certificate Authority"

# Server
openssl genrsa -out server.key 2048
openssl req -new -key server.key -out server.csr -subj "/CN=Server/CN=localhost"
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -days 3650
cat server.key server.crt > server.pem

# Client
openssl genrsa -out client.key 2048
openssl req -new -key client.key -out client.csr -subj "/CN=Client"
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out client.crt -days 3650
cat client.key client.crt > client.pem

# Remove things we don't need
rm ca.key
rm ca.srl
rm client.crt
rm client.csr
rm client.key
rm server.crt
rm server.csr
rm server.key

cd $PWD
