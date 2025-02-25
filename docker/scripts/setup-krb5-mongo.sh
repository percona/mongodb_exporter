#!/bin/bash

username=${MONGO_INITDB_ROOT_USERNAME}
password=${MONGO_INITDB_ROOT_PASSWORD}
port=${PORT:-27017}

docker exec ${KERBEROS_HOST} bash -c "kinit pmm-test@PERCONATEST.COM -kt /tmp/mongodb.keytab"

#docker exec --user root ${MONGO_HOST} bash -c "chown -R mongodb:root /tmp/krb5cc_0"
docker exec --user root ${MONGO_HOST} bash -c "chown -R mongodb:root /tmp/mongodb.keytab"
docker exec ${MONGO_HOST} mongosh "${MONGO_HOST}:${port}" -u ${username} -p ${password} --eval 'db.getSiblingDB("$external").createUser({user: "pmm-test@PERCONATEST.COM",roles: [{role: "read", db: "admin"}]});'
