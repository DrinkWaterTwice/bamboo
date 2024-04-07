#!/usr/bin/env bash

SERVER_PID_FILE=server.pid

SERVER_PID=$(cat "${SERVER_PID_FILE}");

if [ -z "${SERVER_PID}" ]; then
    for id in $(seq 1 4); do
        echo "Process id for server ${id} is written to location: ${SERVER_PID_FILE}"
        ./server -id ${id} -log_dir=. -log_level=debug -algorithm=hotstuff &
        echo $! >> ${SERVER_PID_FILE}
    done
else
    echo "Servers are already started in this folder."
    exit 0
fi

PID_FILE=client.pid

PID=$(cat "${PID_FILE}");

if [ -z "${PID}" ]; then
    echo "Process id for clients is written to location: {$PID_FILE}"
    go build ../client/
    ./client&
    echo $! >> ${PID_FILE}
else
    echo "Clients are already started in this folder."
    exit 0
fi
