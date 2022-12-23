#!/bin/bash

# set proper permissions for secret file, otherwise mongodb won't start
chmod 600 /secret/mongodb_secrets.txt
mongod --port 27017  --oplogSize 16 --bind_ip_all --enableEncryption --encryptionKeyFile  /secret/mongodb_secrets.txt
