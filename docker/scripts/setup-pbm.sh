#!/bin/bash
docker exec -it --user root pbm-mongo-2-1 bash -c "chown -R mongodb /opt/backups"

# PBM config fails if replica sets are not completely up, so give enough time for both replica sets and pbm agents to be up.
sleep 20

docker exec pbm-mongo-2-1 bash -c "pbm config --file /etc/config/pbm.yaml"

# Wait until agents are restarted after config has been updated
sleep 5

docker exec pbm-mongo-2-1 bash -c "pbm backup"

# Wait for backup to complete
sleep 3
