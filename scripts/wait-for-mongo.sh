#!/usr/bin/env sh

waitForMongo(){
    service=$1
    until docker-compose exec $service mongo --quiet --eval 'db.runCommand("ping").ok' > /dev/null; do
    >&2 echo "MongoDB($service) is unavailable - sleeping"
        sleep 1
    done
    >&2 echo "MongoDB($service) is up"
}

for service in mongo mongo-replset
do
    waitForMongo $service
done

