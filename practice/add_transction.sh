#!/usr/bin/env bash
set -e

cd /root/go/src/cothority/conode

# rm -rf `ls -d co*/`
# rm public.toml
rm -rf /root/.local/share/scmgr/config.bin
SCMGR="./scmgr"
$SCMGR link add co1/private.toml
$SCMGR link add co2/private.toml
$SCMGR link add co3/private.toml
$SCMGR skipchain create -b 10 -he 10 co1/public.toml > ./tmp.txt
# while read line; do [ -z "$line" ] && continue ;echo ${line##* }; done < ./tmp.txt 
SKIPCHAINID=`awk 'END {print $NF}' ./tmp.txt`
echo $SKIPCHAINID
./scmgr skipchain block add -roster public.toml $SKIPCHAINID
	