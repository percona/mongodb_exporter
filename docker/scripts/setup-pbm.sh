#!/bin/bash
sleep 3

# We use IP address for the replica set since the docker host name will not be resolvable on the host during tests.
PBM_MONGODB_HOST=$(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' standalone-backup)

docker exec standalone-backup bash -c "
mongo 'mongodb://pbm:pbm@localhost' <<EOF
rs.initiate(
    {
        _id: 'standaloneBackup',
        version: 1,
        members: [

            { _id: 0, \"host\": \"${PBM_MONGODB_HOST}:27017\" }
        ]
    }
)
EOF
"

sleep 3

docker exec percona-backup-mongodb bash -c "pbm config --file /etc/config/pbm.yaml"

# Wait until agents are restarted after config has been updated
sleep 10

docker exec percona-backup-mongodb bash -c "pbm backup"

# Wait for backup to complete
sleep 3
