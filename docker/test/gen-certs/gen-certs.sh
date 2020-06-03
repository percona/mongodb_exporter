#!/bin/bash 

rm *.key *.pem *.crt *.csr

openssl genrsa -out mongodb-test-ca.key 4096
openssl req -new -x509 -days 1826 -key mongodb-test-ca.key -out mongodb-test-ca.crt -config openssl-test-ca.cnf

openssl genrsa -out mongodb-test-ia.key 4096
openssl req -new -key mongodb-test-ia.key -out mongodb-test-ia.csr -config openssl-test-ca.cnf
openssl x509 -sha256 -req -days 730 -in mongodb-test-ia.csr -CA mongodb-test-ca.crt -CAkey mongodb-test-ca.key -set_serial 01 -out mongodb-test-ia.crt -extfile openssl-test-ca.cnf -extensions v3_ca

cat mongodb-test-ca.crt mongodb-test-ia.crt  > test-ca.pem
#cat mongodb-test-ca.crt > test-ca.pem
cp test-ca.pem ../ssl/rootCA.crt

# openssl genrsa -out mongodb-test-server1.key 4096
# openssl req -new -key mongodb-test-server1.key -out mongodb-test-server1.csr -config openssl-test-server.cnf
# openssl x509 -sha256 -req -days 365 -in mongodb-test-server1.csr -CA mongodb-test-ia.crt -CAkey mongodb-test-ia.key -CAcreateserial -out mongodb-test-server1.crt -extfile openssl-test-server.cnf -extensions v3_req
# cat mongodb-test-server1.key mongodb-test-server1.crt > test-server1.pem
# cp test-server1.pem ../ssl/mongodb.pem
# 
# openssl genrsa -out mongodb-test-client.key 4096
# openssl req -new -key mongodb-test-client.key -out mongodb-test-client.csr -config openssl-test-client.cnf
# openssl x509 -sha256 -req -days 365 -in mongodb-test-client.csr -CA mongodb-test-ia.crt -CAkey mongodb-test-ia.key -CAcreateserial -out mongodb-test-client.crt -extfile openssl-test-client.cnf -extensions v3_req
# cat mongodb-test-client.key mongodb-test-client.crt > test-client.pem
# cp test-client.pem ../ssl/client.pem

openssl genrsa -out mongodb-test-server1.key 4096
openssl req -new -key mongodb-test-server1.key -out mongodb-test-server1.csr -config openssl-test-server.cnf
openssl x509 -sha256 -req -days 365 -in mongodb-test-server1.csr -CA mongodb-test-ca.crt -CAkey mongodb-test-ca.key -CAcreateserial -out mongodb-test-server1.crt -extfile openssl-test-server.cnf -extensions v3_req
cat mongodb-test-server1.key mongodb-test-server1.crt > test-server1.pem
cp test-server1.pem ../ssl/mongodb.pem

openssl genrsa -out mongodb-test-client.key 4096
openssl req -new -key mongodb-test-client.key -out mongodb-test-client.csr -config openssl-test-client.cnf
openssl x509 -sha256 -req -days 365 -in mongodb-test-client.csr -CA mongodb-test-ca.crt -CAkey mongodb-test-ca.key -CAcreateserial -out mongodb-test-client.crt -extfile openssl-test-client.cnf -extensions v3_req
cat mongodb-test-client.key mongodb-test-client.crt > test-client.pem
cp test-client.pem ../ssl/client.pem

