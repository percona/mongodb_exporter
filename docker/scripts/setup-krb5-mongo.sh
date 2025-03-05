#!/bin/bash

username=${MONGO_INITDB_ROOT_USERNAME}
password=${MONGO_INITDB_ROOT_PASSWORD}

echo "Waiting for startup.."
until mongosh --host 127.0.0.1:27017 -u ${username} -p ${password} --eval 'quit(db.runCommand({ ping: 1 }).ok ? 0 : 2)' &>/dev/null; do
  printf '.'
  sleep 1
done

echo "Started.."

# create role with anyAction on all resources (needed to allow exporter run execute commands)
# create mongodb user using the same username as the kerberos principal
mongosh --host 127.0.0.1:27017 -u ${username} -p ${password} <<EOF
db.getSiblingDB("admin").createRole({
  role: "anyAction",
  privileges: [
    { resource: { anyResource: true }, actions: [ "anyAction" ] }
  ],
  roles: []
});
db.getSiblingDB("\$external").createUser({user: "pmm-test@PERCONATEST.COM",roles: [{role: "anyAction", db: "admin"}]});
EOF
