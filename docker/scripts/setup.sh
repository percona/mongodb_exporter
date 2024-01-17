#!/bin/bash 

port=${PORT:-27017}

echo "Waiting for startup.."
until mongo --host ${MONGO1}:${port} --eval 'quit(db.runCommand({ ping: 1 }).ok ? 0 : 2)' &>/dev/null; do
  printf '.'
  sleep 1
done

echo "Started.."

echo setup.sh time now: `date +"%T" `


function cnf_servers() {
    echo "setup cnf servers"
    mongo --host ${MONGO1}:${port} <<EOF
    var cfg = {
        "_id": "${RS}",
        "protocolVersion": 1,
        "configsvr": true,
        "members": [
            {
                "_id": 0,
                "host": "${MONGO1}:${port}"
            },
            {
                "_id": 1,
                "host": "${MONGO2}:${port}"
            },
            {
                "_id": 2,
                "host": "${MONGO3}:${port}"
            }
        ]
    };
    rs.initiate(cfg, { force: true });
    rs.reconfig(cfg, { force: true });
EOF
}

function general_servers() {
    echo "setup servers"
    mongo --host ${MONGO1}:${port} <<EOF
    var cfg = {
        "_id": "${RS}",
        "protocolVersion": 1,
        "members": [
            {
                "_id": 0,
                "host": "${MONGO1}:${port}"
            }
        ]
    };
    rs.initiate(cfg, { force: true });
    rs.reconfig(cfg, { force: true });

    rs.add({host:"${MONGO2}", arbiterOnly:false})
    rs.add({host:"${MONGO3}", arbiterOnly:false})
    rs.addArb("${ARBITER}:${port}")
EOF
}


case $1 in
    cnf_servers)
        cnf_servers
        shift
        ;;
    *)
        general_servers
        shift
        ;;
esac
