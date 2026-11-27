#!/bin/bash

username=${MONGO_ADMIN_USERNAME}
password=${MONGO_ADMIN_PASSWORD}

echo "Waiting for startup..."
max_attempts=60
attempts=0
until mongosh --eval 'quit(db.runCommand({ ping: 1 }).ok ? 0 : 2)' &>/dev/null; do
  if [ $attempts -eq $max_attempts ]; then
    echo "Failed to check MongoDB status after $max_attempts attempts"
    exit 1
  fi
  printf '.'
  sleep 1
  attempts=$((attempts+1))
done

echo "Started.."

# The root user is not created via MONGO_INITDB_ROOT_*: that makes the entrypoint run a
# temporary mongod, which can still hold port 27017 when the real one starts (exit 48).
# No user exists yet, so the localhost exception allows creating the first one.
mongosh --eval "db.getSiblingDB('admin').createUser({user: '$username', pwd: '$password', roles: ['root']});"

# create role with anyAction on all resources (needed to allow exporter run execute commands)
# create mongodb user using the same username as the kerberos principal
mongosh -u "$username" -p "$password" --eval 'db.getSiblingDB("admin").createRole({role: "anyAction", privileges: [{ resource: { anyResource: true }, actions: [ "anyAction" ] }], roles: [] });'
mongosh -u "$username" -p "$password" --eval 'db.getSiblingDB("$external").createUser({user: "pmm-test@PERCONATEST.COM", roles: [{role: "anyAction", db: "admin"}]});'
