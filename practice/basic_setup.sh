#!/usr/bin/env bash
set -e

n=1
start=$(date +%s%N)/1000000
while [ $n -le 3 ] 
do
    docker start box$n
    docker exec -d box$n /bin/bash -c 'chmod +x setup.sh  && ./setup.sh'
    ((n++))
done
    
end=$(date +%s%N)/1000000
take=$(( end - start ))
echo Time taken to execute commands is ${take}