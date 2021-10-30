#!/usr/bin/env bash
set -e

n=1
while [ $n -le 3 ] 
do
    docker cp box$n:/root/go/src/cothority/conode/co$n /root/go/src/cothority/conode
    # cd /root/go/src/cothority/conode
    # cat co$n/public.toml >> /root/go/src/cothority/conode/public.toml
    ((n++))
done

cd /root/go/src/cothority/conode
cat co*/public.toml > ./public.toml


# cat tmp.txt | awk 'NR==3 || NR==5' | awk '{print $NF}' > new.txt