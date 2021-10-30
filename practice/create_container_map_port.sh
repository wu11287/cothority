#!/usr/bin/env bash
set -e

port=7770
port2=7771
cnt=1
end=3

while [ $cnt -le $end ] 
do
    docker run -itd --env PORT_ENV=$port --env CNT_ENV=$cnt --name box$cnt --network my-net -p $port-$port2:$port-$port2 basic_node /bin/bash
    let port=port2+1
    let port2=port+1
    let cnt=cnt+1
done