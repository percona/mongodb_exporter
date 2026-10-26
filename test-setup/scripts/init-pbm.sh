#!/bin/bash
docker exec --user root pbm-mongo-2-1 bash -c "chown -R mongodb /opt/backups"

# PBM config fails if replica sets are not completely up, so give enough time for both replica sets and pbm agents to be up.
sleep 20

docker exec pbm-mongo-2-1 bash -c "pbm config --file /etc/config/pbm.yaml"

# Wait until agents are restarted after config has been updated
sleep 5

# The agents accept a backup only after they have checked the new storage, so retry until one is taken.
for _ in $(seq 1 12); do
    docker exec pbm-mongo-2-1 bash -c "pbm backup --wait" && break
    sleep 5
done
