#!/usr/bin/env bash
set -e

basic_port=7770
basic_cnt=1

export PORT_ENV=$basic_port
export CNT_ENV=$basic_cnt
cd /root/go/src/cothority/conode
./run_nodes.sh
exit