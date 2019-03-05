#!/usr/bin/env sh

until docker-compose exec mongo mongo --quiet --eval 'db.runCommand("ping").ok' > /dev/null; do
>&2 echo "MongoDB is unavailable - sleeping"
    sleep 1
done
>&2 echo "MongoDB is up"
