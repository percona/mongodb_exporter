#!/bin/bash 

TOP_DIR=$(git rev-parse --show-toplevel)
cd ${TOP_DIR}/docker/test/gen-certs/
                                           				   	
CURDIR=$(pwd)
mkdir -p ${TOP_DIR}/docker/test/ssl/
echo "Generating certs into $CURDIR"
rm -f *.key *.pem *.crt *.csr

openssl genrsa -out mongodb-test-ca.key 4096
openssl req -new -x509 -days 1826 -key mongodb-test-ca.key -out mongodb-test-ca.crt -config openssl-test-ca.cnf \
    -subj "/C=US/ST=Denial/L=Springfield/O=Dis/CN=www.example.com"

openssl genrsa -out mongodb-test-ia.key 4096
openssl req -new -key mongodb-test-ia.key -out mongodb-test-ia.csr -config openssl-test-ca.cnf \
    -subj "/C=US/ST=Denial/L=Springfield/O=Dis/CN=www.example.com"

openssl x509 -sha256 -req -days 730 -in mongodb-test-ia.csr -CA mongodb-test-ca.crt -CAkey mongodb-test-ca.key -set_serial 01 -out mongodb-test-ia.crt -extfile openssl-test-ca.cnf -extensions v3_ca 

cat mongodb-test-ca.crt mongodb-test-ia.crt  > test-ca.pem
cp test-ca.pem ${TOP_DIR}/docker/test/ssl/rootCA.crt

openssl genrsa -out mongodb-test-server1.key 4096

openssl req -new -key mongodb-test-server1.key -out mongodb-test-server1.csr -config openssl-test-server.cnf \
    -subj "/C=US/ST=Denial/L=Springfield/O=Dis/CN=www.example.com"

openssl x509 -sha256 -req -days 365 -in mongodb-test-server1.csr -CA mongodb-test-ca.crt -CAkey mongodb-test-ca.key -CAcreateserial -out mongodb-test-server1.crt -extfile openssl-test-server.cnf -extensions v3_req 
cat mongodb-test-server1.key mongodb-test-server1.crt > test-server1.pem
cp test-server1.pem ${TOP_DIR}/docker/test/ssl/mongodb.pem

openssl genrsa -out mongodb-test-client.key 4096
openssl req -new -key mongodb-test-client.key -out mongodb-test-client.csr -config openssl-test-client.cnf \
    -subj "/C=US/ST=Denial/L=Springfield/O=Dis/CN=www.example.com"
openssl x509 -sha256 -req -days 365 -in mongodb-test-client.csr -CA mongodb-test-ca.crt -CAkey mongodb-test-ca.key -CAcreateserial -out mongodb-test-client.crt -extfile openssl-test-client.cnf -extensions v3_req 
cat mongodb-test-client.key mongodb-test-client.crt > test-client.pem
cp test-client.pem ${TOP_DIR}/docker/test/ssl/client.pem
# clean up
rm -f *.key *.pem *.crt *.csr
