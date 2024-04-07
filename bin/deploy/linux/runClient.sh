#!/usr/bin/env bash

if [ $# -eq 0 ]; then
    echo "Please provide the number of clients as a command-line argument."
    exit 1
fi

go build ../client/
int=1
while (( $int<=$1 ))
do
    ./client&
    let "int++"
done
echo "$1 clients are started"
