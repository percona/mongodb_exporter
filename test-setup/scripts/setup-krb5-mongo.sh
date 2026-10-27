#!/bin/bash

username=${MONGO_INITDB_ROOT_USERNAME}
password=${MONGO_INITDB_ROOT_PASSWORD}

# MONGO_INITDB_ROOT_USERNAME makes the entrypoint run a temporary mongod bound to
# 127.0.0.1, create the root user and shut it down before starting the real server.
# Probing the container's own address skips that instance, so the commands below
# cannot land on a server that is about to disappear.
host=$(getent ahostsv4 "$HOSTNAME" | awk '{ print $1; exit }')

echo "Waiting for startup on ${host}:27017..."
max_attempts=60
attempts=0
until mongosh --host ${host}:27017 -u ${username} -p ${password} --eval 'quit(db.runCommand({ ping: 1 }).ok ? 0 : 2)' &>/dev/null; do
  if [ $attempts -eq $max_attempts ]; then
    echo "Failed to check MongoDB status after $max_attempts attempts"
    exit 1
  fi
  printf '.'
  sleep 1
  attempts=$((attempts+1))
done

echo "Started.."

# create role with anyAction on all resources (needed to allow exporter run execute commands)
# create mongodb user using the same username as the kerberos principal
mongosh --host ${host}:27017 -u "$username" -p "$password" --eval 'db.getSiblingDB("admin").createRole({role: "anyAction", privileges: [{ resource: { anyResource: true }, actions: [ "anyAction" ] }], roles: [] });'
mongosh --host ${host}:27017 -u "$username" -p "$password" --eval 'db.getSiblingDB("$external").createUser({user: "pmm-test@PERCONATEST.COM", roles: [{role: "anyAction", db: "admin"}]});'
