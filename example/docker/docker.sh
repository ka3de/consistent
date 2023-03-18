#!/bin/bash

set -e

docker build ../.. -f ../Dockerfile -t ka3de/consistent

# start two nodes
echo "start two nodes with load balancer"
docker-compose up --scale consistent=2 -d
sleep 2

# make some requests to HTTP API
echo "one request to get current snapshot"
curl http://127.0.0.1:8081/consistent/snapshot
echo -e "\n"
sleep 1

echo "another request to get current snapshot"
curl http://127.0.0.1:8081/consistent/snapshot
echo -e "\n"
sleep 1

echo "one request for key='test'"
curl http://127.0.0.1:8081/consistent/srv?key=test
echo -e "\n"
sleep 1

echo "another request for key='test'"
curl http://127.0.0.1:8081/consistent/srv?key=test
echo -e "\n"
sleep 1

echo "one request for key='aDifferentKey'"
curl http://127.0.0.1:8081/consistent/srv?key=aDifferentKey
echo -e "\n"
sleep 1

sleep 2

# add one node
echo "add one node"
docker-compose up -d --scale consistent=3 --no-recreate

# make some requests to HTTP API
echo "one request to get current snapshot"
curl http://127.0.0.1:8081/consistent/snapshot
echo -e "\n"
sleep 1

echo "another request to get current snapshot"
curl http://127.0.0.1:8081/consistent/snapshot
echo -e "\n"
sleep 1

echo "one request for key='test'"
curl http://127.0.0.1:8081/consistent/srv?key=test
echo -e "\n"
sleep 1

echo "another request for key='test'"
curl http://127.0.0.1:8081/consistent/srv?key=test
echo -e "\n"
sleep 1

# kill one node
echo "killing one node"
docker-compose up -d --scale consistent=2 --no-recreate
echo -e "\n"
sleep 1

# make some requests to HTTP API
echo "one request to get current snapshot"
curl http://127.0.0.1:8081/consistent/snapshot
echo -e "\n"
sleep 1

echo "another request to get current snapshot"
curl http://127.0.0.1:8081/consistent/snapshot
echo -e "\n"
sleep 1

echo "one request for key='test'"
curl http://127.0.0.1:8081/consistent/srv?key=test
echo -e "\n"
sleep 1

echo "another request for key='test'"
curl http://127.0.0.1:8081/consistent/srv?key=test
echo -e "\n"
sleep 1

# kill another node
echo "killing one node"
docker-compose up -d --scale consistent=1 --no-recreate
echo -e "\n"
sleep 1

# make one more request to HTTP API
echo "one request to get current snapshot"
curl http://127.0.0.1:8081/consistent/snapshot
echo -e "\n"
sleep 1

echo "another request for key='test'"
curl http://127.0.0.1:8081/consistent/srv?key=test
echo -e "\n"
sleep 1

# finish
echo "done"
docker-compose stop
