#!/bin/bash

docker exec --user root psmdb-kerberos bash -c 'chown mongodb:root /krb5/mongodb.keytab'
docker exec psmdb-kerberos bash -c '/scripts/setup-krb5-mongo.sh'
