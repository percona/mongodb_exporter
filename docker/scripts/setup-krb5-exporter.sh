#!/usr/bin/env sh

mongohost=`getent hosts ${MONGO_HOST} | awk '{ print $1 }'`

username=${MONGO_KERBEROS_USERNAME}
password=${MONGO_KERBEROS_PASSWORD}

export MONGODB_URI="mongodb://${username}:${password}@${mongohost}:27017/?directConnection=true&authSource=%24external&authMechanism=GSSAPI"

/mongodb_exporter "$@"