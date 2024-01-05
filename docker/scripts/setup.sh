#!/bin/bash 
readarray -d . -t verarr <<< "${VERSION}"
readarray -d : -t version <<< "${verarr[0]}"
echo "Mongo version: ${version[1]}"

if ((${version[1]} <= 4))
then
  echo -e 'mongo' > /usr/bin/mongosh && chmod +x /usr/bin/mongosh
fi

mongodb1=`getent hosts ${MONGO1} | awk '{ print $1 }'`
mongodb2=`getent hosts ${MONGO2} | awk '{ print $1 }'`
mongodb3=`getent hosts ${MONGO3} | awk '{ print $1 }'`
arbiter=`getent hosts ${ARBITER} | awk '{ print $1 }'`

port=${PORT:-27017}

echo "Waiting for startup.."
until mongosh --host ${mongodb1}:${port} --eval 'quit(db.runCommand({ ping: 1 }).ok ? 0 : 2)' &>/dev/null; do
  printf '.'
  sleep 1
done

echo "Started.."

echo setup.sh time now: `date +"%T" `


function cnf_servers() {
    echo "setup cnf servers on ${MONGO1}(${mongodb1}:${port})"
    mongosh --host ${mongodb1}:${port} <<EOF
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
            {
                "_id": 1,
                "host": "${mongodb2}:${port}"
            },
            {
                "_id": 2,
                "host": "${mongodb3}:${port}"
            },
        ]
    };

    rs.initiate(cfg);
EOF
}

function general_servers() {
    echo "setup servers on ${MONGO1}(${mongodb1}:${port})"
    mongosh --host ${mongodb1}:${port} <<EOF
    var cfg = {
        "_id": "${RS}",
        "protocolVersion": 1,
        "version": 1,
        "members": [
            {
                "_id": 0,
                "host": "${mongodb1}:${port}"
            },
            {
                "_id": 1,
                "host": "${mongodb2}:${port}"
            },
            {
                "_id": 2,
                "host": "${mongodb3}:${port}"
            },
            {
                "_id": 3,
                "host": "${arbiter}:${port}",
                "arbiterOnly": true
            },
        ]
    };
    rs.initiate(cfg);
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
    *)
        general_servers
        shift
        ;;
esac
