#!/bin/bash

cp /mongos.key /tmp/mongos.key
cp /mongos.pem /tmp/mongos.pem
cp /rootCA.crt /tmp/mongod-rootCA.crt
chmod 400 /tmp/mongos.key /tmp/mongos.pem /tmp/mongod-rootCA.crt

/usr/bin/mongos \
  --bind_ip=0.0.0.0 \
  --sslMode=preferSSL \
  --sslCAFile=/tmp/mongod-rootCA.crt \
  --sslPEMKeyFile=/tmp/mongos.pem \
  --sslAllowInvalidHostnames \
  --sslAllowInvalidCertificates \
  $*
