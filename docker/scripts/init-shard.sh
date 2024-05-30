#!/bin/bash 
# `mongosh` is used starting from MongoDB 5.x
MONGODB_CLIENT="mongosh --quiet"
PARSED=(${VERSION//:/ })
MONGODB_VERSION=${PARSED[1]}
MONGODB_VENDOR=${PARSED[0]}
if [ "`echo ${MONGODB_VERSION} | cut -c 1`" = "4" ]; then
  MONGODB_CLIENT="mongo"
fi
if [ "`echo ${MONGODB_VERSION} | cut -c 1`" = "5" ] && [ ${MONGODB_VENDOR} == "percona/percona-server-mongodb" ]; then
  MONGODB_CLIENT="mongo"
fi
echo "MongoDB vendor, client and version: ${MONGODB_VENDOR} ${MONGODB_CLIENT} ${MONGODB_VERSION}"

mongodb1=`getent hosts ${MONGOS} | awk '{ print $1 }'`

mongodb11=`getent hosts ${MONGO11} | awk '{ print $1 }'`
mongodb12=`getent hosts ${MONGO12} | awk '{ print $1 }'`
mongodb13=`getent hosts ${MONGO13} | awk '{ print $1 }'`

mongodb21=`getent hosts ${MONGO21} | awk '{ print $1 }'`
mongodb22=`getent hosts ${MONGO22} | awk '{ print $1 }'`
mongodb23=`getent hosts ${MONGO23} | awk '{ print $1 }'`

mongodb31=`getent hosts ${MONGO31} | awk '{ print $1 }'`
mongodb32=`getent hosts ${MONGO32} | awk '{ print $1 }'`
mongodb33=`getent hosts ${MONGO33} | awk '{ print $1 }'`

port=${PORT:-27017}

echo "Waiting for startup.."
until ${MONGODB_CLIENT} --host ${mongodb1}:${port} --eval 'quit(db.runCommand({ ping: 1 }).ok ? 0 : 2)' &>/dev/null; do
  printf '.'
  sleep 1
done

echo "Started.."

echo init-shard.sh time now: `date +"%T" `
echo "Configuring sharding.."
echo  "${RS1}/${mongodb11}:${PORT1},${mongodb12}:${PORT2},${mongodb13}:${PORT3}"
echo  "${RS2}/${mongodb21}:${PORT1},${mongodb22}:${PORT2},${mongodb23}:${PORT3}"

${MONGODB_CLIENT} --host ${mongodb1}:${port} <<EOF
   sh.addShard( "${RS1}/${mongodb11}:${PORT1},${mongodb12}:${PORT2},${mongodb13}:${PORT3}" );
   sh.addShard( "${RS2}/${mongodb21}:${PORT1},${mongodb22}:${PORT2},${mongodb23}:${PORT3}" );
   use test;
   db.createCollection("shard");
   sh.enableSharding("test");
   sh.shardCollection( "test.shard", { id: "hashed" }, false, { numInitialChunks: 500, collation: { locale: "simple" }} );
   sh.status();
EOF
