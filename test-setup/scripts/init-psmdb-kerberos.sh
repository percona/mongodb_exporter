#!/bin/bash
set -eu

# wait_for retries a command until it succeeds. On timeout it prints the container's log,
# since the alternative is a bare non-zero exit that says nothing about what went wrong.
wait_for() {
    local what=$1 container=$2
    shift 2

    echo "Waiting for ${what}..."
    for _ in $(seq 60); do
        if "$@" > /dev/null 2>&1; then
            echo "${what} is ready"
            return
        fi
        printf '.'
        sleep 2
    done

    echo "timed out waiting for ${what}"
    docker logs --tail 30 "${container}" || true
    exit 1
}

is_healthy() {
    [ "$(docker inspect -f '{{.State.Health.Status}}' "$1")" = "healthy" ]
}

# `docker compose up -d` returns as soon as the containers are created, so both of the execs
# below used to run against a server that was still starting -- or, when the entrypoint lost
# its bind race, against one that had already exited, which is a docker exec that dies with
# 137 and takes the whole cluster setup down with it.
wait_for "psmdb-kerberos" psmdb-kerberos is_healthy psmdb-kerberos

# The KDC writes the keytab as its last step before it starts serving, so its own healthcheck
# goes green a moment before the file the chown needs exists.
wait_for "the kerberos keytab" kerberos docker exec psmdb-kerberos test -f /krb5/mongodb.keytab

docker exec --user root psmdb-kerberos bash -c 'chown mongodb:root /krb5/mongodb.keytab'
docker exec psmdb-kerberos bash -c '/scripts/setup-krb5-mongo.sh'
