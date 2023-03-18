#!/bin/bash

set -e

cd .. && go build -o node

# start nodes
./node -api-port 3334 -port 3333 &
NODE_1_PID=$!
echo "started node_1 with pid $NODE_1_PID"

./node -api-port 4445 -port 4444 -nodelist 127.0.0.1:3333 &
NODE_2_PID=$!
echo "started node_2 with pid $NODE_2_PID"

./node -api-port 5556 -port 5555 -nodelist 127.0.0.1:3333,127.0.0.1:4444 &
NODE_3_PID=$!
echo "started node_3 with pid $NODE_3_PID"

sleep 2

# make some requests to HTTP API
echo "current snapshot according to node 1"
curl http://127.0.0.1:3334/consistent/snapshot
echo -e "\n"
sleep 1

echo "current snapshot according to node 3"
curl http://127.0.0.1:5556/consistent/snapshot
echo -e "\n"
sleep 1

echo "srv for key='test' according to node 2"
curl http://127.0.0.1:4445/consistent/srv?key=test
echo -e "\n"
sleep 1

echo "srv for key='test' according to node 1"
curl http://127.0.0.1:3334/consistent/srv?key=test
echo -e "\n"
sleep 1

echo "srv for key='aDifferentKey' according to node 3"
curl http://127.0.0.1:5556/consistent/srv?key=aDifferentKey
echo -e "\n"

sleep 2

# kill one node
echo "killing node 2"
kill -SIGINT $NODE_2_PID
echo -e "\n"
sleep 1

# make some more requests to HTTP API
echo "current snapshot according to node 1"
curl http://127.0.0.1:3334/consistent/snapshot
echo -e "\n"
sleep 1

echo "current snapshot according to node 3"
curl http://127.0.0.1:5556/consistent/snapshot
echo -e "\n"
sleep 1

sleep 2

# kill another node
echo "killing node 1"
kill $NODE_1_PID
echo -e "\n"
sleep 1

# make one more request to HTTP API
echo "current snapshot according to node 3"
curl http://127.0.0.1:5556/consistent/snapshot
echo -e "\n"
sleep 1

echo "srv for key='test' according to node 3"
curl http://127.0.0.1:5556/consistent/srv?key=test
echo -e "\n"
sleep 1

# kill last node
echo "killing node 3"
kill -SIGINT $NODE_3_PID
echo -e "\n"

# finish
echo "done"
