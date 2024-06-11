#!/bin/bash
sleep 3
docker exec standalone-backup bash -c "
mongo 'mongodb://pbm:pbm@localhost' <<EOF
rs.initiate(
    {
        _id: 'standaloneBackup',
        version: 1,
        members: [

            { _id: 0, \"host\": \"standalone-backup:27017\" }
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
