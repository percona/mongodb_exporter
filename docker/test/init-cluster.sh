#!/bin/bash

max_tries=45
sleep_secs=1

cp /rootCA.crt /tmp/rootCA.crt
cp /client.pem /tmp/client.pem
chmod 400 /tmp/rootCA.crt /tmp/client.pem

MONGO_FLAGS="--quiet"

sleep $sleep_secs

/usr/bin/mongo --version

## Configsvr replset
tries=1
while [ $tries -lt $max_tries ]; do
	/usr/bin/mongo ${MONGO_FLAGS} \
	  --host=configsvr1 \
	  --port=27019 \
		--eval='rs.initiate({
				_id: "'${TEST_MONGODB_CONFIGSVR_RS}'",
				configsvr: true,
				version: 1,
				members: [
					{ _id: 0, host: "configsvr1:27019" }
				]
		})'
	[ $? == 0 ] && break
	echo "# INFO: retrying rs.initiate() for configsvr in $sleep_secs secs (try $tries/$max_tries)"
	sleep $sleep_secs
	tries=$(($tries + 1))
done
if [ $tries -ge $max_tries ]; then
	echo "# ERROR: reached max tries $max_tries, exiting"
	exit 1
fi
echo "# INFO: sharding configsvr is initiated"

## Shard 1
tries=1
while [ $tries -lt $max_tries ]; do
	/usr/bin/mongo ${MONGO_FLAGS} \
	  --host=s1-mongo1 \
	  --port=27018 \
		--eval='rs.initiate({
			_id: "'${TEST_MONGODB_S1_RS}'",
			version: 1,
			members: [
				{ _id: 0, host: "s1-mongo1:27018", priority: 10 },
				{ _id: 1, host: "s1-mongo2:27018", priority: 1 },
				{ _id: 2, host: "s1-mongo3:27018", priority: 0, hidden: true, tags: { role: "backup" } }
			]})' | tee /tmp/init-result.json
	if [ $? == 0 ]; then
	  grep -q '"ok" : 1' /tmp/init-result.json
	  [ $? == 0 ] && rm -vf /tmp/init-result.json && break
	fi
	echo "# INFO: retrying rs.initiate() on ${TEST_MONGODB_S1_RS} in $sleep_secs secs (try $tries/$max_tries)"
	sleep $sleep_secs
	tries=$(($tries + 1))
done
if [ $tries -ge $max_tries ]; then
	echo "# ERROR: reached max tries $max_tries for ${TEST_MONGODB_S1_RS}, exiting"
	exit 1
fi
echo "# INFO: replset ${TEST_MONGODB_S1_RS} is initiated"


## Shard 2
tries=1
while [ $tries -lt $max_tries ]; do
	/usr/bin/mongo ${MONGO_FLAGS} \
	  --host=s2-mongo1 \
	  --port=27018 \
		--eval='rs.initiate({
			_id: "'${TEST_MONGODB_S2_RS}'",
			version: 1,
			members: [
				{ _id: 0, host: "s2-mongo1:27018", priority: 10 },
				{ _id: 1, host: "s2-mongo2:27018", priority: 1 },
				{ _id: 2, host: "s2-mongo3:27018", priority: 0, hidden: true, tags: { role: "backup" } }
			]})' | tee /tmp/init-result.json
	if [ $? == 0 ]; then
	  grep -q '"ok" : 1' /tmp/init-result.json
	  [ $? == 0 ] && rm -vf /tmp/init-result.json && break
	fi
	echo "# INFO: retrying rs.initiate() on ${TEST_MONGODB_S2_RS} in $sleep_secs secs (try $tries/$max_tries)"
	sleep $sleep_secs
	tries=$(($tries + 1))
done
if [ $tries -ge $max_tries ]; then
	echo "# ERROR: reached max tries $max_tries for ${TEST_MONGODB_S2_RS}, exiting"
	exit 1
fi
echo "# INFO: replset ${TEST_MONGODB_S2_RS} is initiated"

## Replica set 3 (non sharded)
tries=1
while [ $tries -lt $max_tries ]; do
	/usr/bin/mongo ${MONGO_FLAGS} \
	  --host=s3-mongo1 \
	  --port=27017 \
		--eval='rs.initiate({
			_id: "'${TEST_MONGODB_S3_RS}'",
			version: 1,
			members: [
				{ _id: 0, host: "s3-mongo1:27017", priority: 10 },
				{ _id: 1, host: "s3-mongo2:27017", priority: 1 },
				{ _id: 2, host: "s3-mongo3:27017", priority: 0, hidden: true, tags: { role: "backup" } }
			]})' | tee /tmp/init3-result.json
	if [ $? == 0 ]; then
	  grep -q '"ok" : 1' /tmp/init3-result.json
	  [ $? == 0 ] && rm -vf /tmp/init3-result.json && break
	fi
	echo "# INFO: retrying rs.initiate() on ${TEST_MONGODB_S3_RS} in $sleep_secs secs (try $tries/$max_tries)"
	sleep $sleep_secs
	tries=$(($tries + 1))
done
if [ $tries -ge $max_tries ]; then
	echo "# ERROR: reached max tries $max_tries for ${TEST_MONGODB_S3_RS}, exiting"
	exit 1
fi
echo "# INFO: replset ${TEST_MONGODB_S3_RS} is initiated"

for MONGODB_ADDRESS in s1-mongo1:27018 s2-mongo1:27018 configsvr1:27019; do
	tries=1
	while [ $tries -lt $max_tries ]; do
		ISMASTER=$(/usr/bin/mongo ${MONGO_FLAGS} \
			--host=$(echo $MONGODB_ADDRESS | cut -d: -f1) \
			--port=$(echo $MONGODB_ADDRESS | cut -d: -f2) \
			--eval='printjson(db.isMaster().ismaster)' 2>/dev/null)
		[ "$ISMASTER" == "true" ] && break
		echo "# INFO: retrying db.isMaster() check on ${MONGODB_ADDRESS} in $sleep_secs secs (try $tries/$max_tries)"
		sleep $sleep_secs
		tries=$(($tries + 1))
	done
	if [ $tries -ge $max_tries ]; then
		echo "# ERROR: reached max tries $max_tries, exiting"
		exit 1
	fi
done
echo "# INFO: all replsets have primary"

## Add authentication for master hosts.
for MONGODB_ADDRESS in standalone:27017 s1-mongo1:27018 s2-mongo1:27018 configsvr1:27019 s3-mongo1:27017; do
    echo "ADDRESS $MONGODB_ADDRESS"
	tries=1
	while [ $tries -lt $max_tries ]; do
		/usr/bin/mongo ${MONGO_FLAGS} \
			--host=$(echo $MONGODB_ADDRESS | cut -d: -f1) \
			--port=$(echo $MONGODB_ADDRESS | cut -d: -f2) \
			--eval='db.createUser({
				user: "'${TEST_MONGODB_ADMIN_USERNAME}'",
				pwd: "'${TEST_MONGODB_ADMIN_PASSWORD}'",
				roles: ["root"]
			})' \
			admin
		if [ $? == 0 ]; then
			echo "# INFO: added admin user to ${MONGODB_ADDRESS}"
			/usr/bin/mongo ${MONGO_FLAGS} \
				--username=${TEST_MONGODB_ADMIN_USERNAME} \
				--password=${TEST_MONGODB_ADMIN_PASSWORD} \
			  --host=$(echo $MONGODB_ADDRESS | cut -d: -f1) \
			  --port=$(echo $MONGODB_ADDRESS | cut -d: -f2) \
				--eval='db.createUser({
					user: "'${TEST_MONGODB_USERNAME}'",
					pwd: "'${TEST_MONGODB_PASSWORD}'",
					roles: [
						{ db: "admin", role: "backup" },
						{ db: "admin", role: "clusterMonitor" },
						{ db: "admin", role: "restore" },
						{ db: "config", role: "read" },
						{ db: "test", role: "readWrite" }
					]
				})' \
				admin
			[ $? == 0 ] && echo "# INFO: added test user to ${MONGODB_ADDRESS}" && break
		fi
		echo "# INFO: retrying db.createUser() on ${MONGODB_ADDRESS} in $sleep_secs secs (try $tries/$max_tries)"
		sleep $sleep_secs
		tries=$(($tries + 1))
	done
done
echo "# INFO: all replsets have auth user(s)"


shard1=${TEST_MONGODB_S1_RS}'/s1-mongo1:27018,s1-mongo2:27018'
shard2=${TEST_MONGODB_S2_RS}'/s2-mongo1:27018,s2-mongo2:27018'
for shard in $shard1 $shard2; do
	tries=1
	while [ $tries -lt $max_tries ]; do
		ADDSHARD=$(/usr/bin/mongo ${MONGO_FLAGS} \
			--username=${TEST_MONGODB_ADMIN_USERNAME} \
			--password=${TEST_MONGODB_ADMIN_PASSWORD} \
			--host=mongos \
			--port=27017 \
			--eval='printjson(sh.addShard("'$shard'").ok)' \
			admin 2>/dev/null)
		[ $? == 0 ] && [ "$ADDSHARD" == "1" ] && break
		echo "# INFO: retrying sh.addShard() check for '$shard' in $sleep_secs secs (try $tries/$max_tries)"
		sleep $sleep_secs
		tries=$(($tries + 1))
	done
	if [ $tries -ge $max_tries ]; then
		echo "# ERROR: reached max tries $max_tries for '$shard', exiting"
		exit 1
	fi
	echo "# INFO: added shard: $shard"
done

tries=1
while [ $tries -lt $max_tries ]; do
	ENABLESHARDING=$(/usr/bin/mongo ${MONGO_FLAGS} \
		--username=${TEST_MONGODB_ADMIN_USERNAME} \
		--password=${TEST_MONGODB_ADMIN_PASSWORD} \
		--host=mongos \
		--port=27017 \
		--eval='sh.enableSharding("test").ok' \
		admin 2>/dev/null)
	[ $? == 0 ] && [ "$ENABLESHARDING" == "1" ] && break
	echo "# INFO: retrying sh.enableSharding(\"test\") check in $sleep_secs secs (try $tries/$max_tries)"
	sleep $sleep_secs
	tries=$(($tries + 1))
done
if [ $tries -ge $max_tries ]; then
	echo "# ERROR: reached max tries $max_tries for '$shard', exiting"
	exit 1
fi
echo "# INFO: \"test\" database is now sharded"

tries=1
while [ $tries -lt $max_tries ]; do
	SHARDCOL=$(/usr/bin/mongo ${MONGO_FLAGS} \
		--username=${TEST_MONGODB_ADMIN_USERNAME} \
		--password=${TEST_MONGODB_ADMIN_PASSWORD} \
		--host=mongos \
		--port=27017 \
		--eval='sh.shardCollection("test.test", {_id: 1}).ok' \
		admin 2>/dev/null)
	[ $? == 0 ] && [ "$ENABLESHARDING" == "1" ] && break
	echo "# INFO: retrying sh.shardCollection(\"test.test\", {_id: 1}) check in $sleep_secs secs (try $tries/$max_tries)"
	sleep $sleep_secs
	tries=$(($tries + 1))
done
if [ $tries -ge $max_tries ]; then
	echo "# ERROR: reached max tries $max_tries for '$shard', exiting"
	exit 1
fi
echo "# INFO: \"test.test\" collection is now sharded"
