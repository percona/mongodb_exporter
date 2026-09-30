#!/bin/bash
set -eu

# wait_for retries a command until it succeeds. It gives up early once psmdb-kerberos has
# stopped restarting and stayed dead, since nothing it waits for can happen after that, and on
# either exit it prints both containers' logs -- the alternative is a bare non-zero exit that
# says nothing about what went wrong.
wait_for() {
    local what=$1
    shift

    echo "Waiting for ${what}..."
    for _ in $(seq 60); do
        if "$@" > /dev/null 2>&1; then
            echo "${what} is ready"
            return
        fi
        if [ "$(docker inspect -f '{{.State.Status}}' psmdb-kerberos 2>/dev/null)" = "exited" ]; then
            echo "psmdb-kerberos exited while waiting for ${what}"
            break
        fi
        printf '.'
        sleep 2
    done

    echo "gave up waiting for ${what}"
    for container in psmdb-kerberos kerberos; do
        echo "--- ${container}"
        docker logs --tail 30 "${container}" || true
    done
    exit 1
}

is_healthy() {
    [ "$(docker inspect -f '{{.State.Health.Status}}' "$1")" = "healthy" ]
}

# `docker compose up -d` returns as soon as the containers are created, so both of the execs
# below used to run against a server that was still starting -- or, when the entrypoint lost
# its bind race, against one that had already exited, which is a docker exec that dies with
# 137 and takes the whole cluster setup down with it.
wait_for "psmdb-kerberos" is_healthy psmdb-kerberos

# The KDC creates its database, adds three principals and only then exports the keytab, so its
# own healthcheck -- which passes as soon as the database exists -- goes green well before the
# file the chown needs.
wait_for "the kerberos keytab" docker exec psmdb-kerberos test -f /krb5/mongodb.keytab

docker exec --user root psmdb-kerberos bash -c 'chown mongodb:root /krb5/mongodb.keytab'
docker exec psmdb-kerberos bash -c '/scripts/setup-krb5-mongo.sh'
