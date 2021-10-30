#!/usr/bin/env bash
set -e

GO=/usr/bin/go


start=$(date +%s%N)/1000000
# start=$start/1000000
$GO run ./pos/pos.go > ./tmp.txt
ischoosed=$(cat ./tmp.txt)
if [ $ischoosed="true" ]; then
    echo "11111"
fi

end=$(date +%s%N)/1000000
# end=$end/1000000
take=$(( end - start ))
echo Time taken to execute commands is ${take} ms 
