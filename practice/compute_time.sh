#!/usr/bin/env bash
set -e

# i=1
# while [ $i -le 20 ]
# do
#     # docker exec -d box$i /bin/bash -c 'cd /root/go/src/cothority/conode && cat tmp.txt | awk 'NR==1 || NR==3' | awk '{print $NF}' > new.txt'
#     docker cp box$i:/root/go/src/cothority/conode/tmp.txt tmp$i.txt
#     # docker cp box$i:/root/go/src/cothority/conode/co$i /root/go/src/cothority/conode
#     ((i++))
# done

i=1
while [ $i -le 20 ]
do
    # cat tmp$i.txt | awk 'NR!=3' > t$i.txt
    # rm tmp$i.txt
    cat tmp$i.txt | awk 'NR==4' | awk '{print $(NF-1)}' >> all.txt
    ((i++))
done