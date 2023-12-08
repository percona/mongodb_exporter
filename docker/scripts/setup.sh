#!/bin/bash 

mongodb1=`getent hosts ${MONGO1} | awk '{ print $1 }'`
mongodb2=`getent hosts ${MONGO2} | awk '{ print $1 }'`
mongodb3=`getent hosts ${MONGO3} | awk '{ print $1 }'`
arbiter=`getent hosts ${ARBITER} | awk '{ print $1 }'`

port=${PORT:-27017}

echo "Waiting for startup.."
until mongo --host ${mongodb1}:${port} --eval 'quit(db.runCommand({ ping: 1 }).ok ? 0 : 2)' &>/dev/null; do
  printf '.'
  sleep 1
done

echo "Started.."

echo setup.sh time now: `date +"%T" `


function cnf_servers() {
    echo "setup cnf servers on ${MONGO1}(${mongodb1}:${port})"
    mongo --host ${mongodb1}:${port} <<EOF
    var cfg = {
        "_id": "${RS}",
        "version": 1,
        "protocolVersion": 1,
        "configsvr": true,
        "members": [
            {
                "_id": 0,
                "host": "${mongodb1}:${port}"
            },
        ]
    };

    rs.initiate(cfg);
    rs.add({"_id": 1, "host": "${mongodb2}:${port}"});
    rs.add({"_id": 2, "host": "${mongodb3}:${port}"});
EOF
}

function general_servers() {
    echo "setup servers on ${MONGO1}(${mongodb1}:${port})"
    mongo --host ${mongodb1}:${port} <<EOF
    var cfg = {
        "_id": "${RS}",
        "protocolVersion": 1,
        "version": 1,
        "members": [
            {
                "_id": 0,
                "host": "${mongodb1}:${port}"
            },
        ]
    };
    rs.initiate(cfg);
    rs.add({"_id": 1, "host": "${mongodb2}:${port}"});
    rs.add({"_id": 2, "host": "${mongodb3}:${port}"});
    rs.addArb("${arbiter}:${port}");
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
