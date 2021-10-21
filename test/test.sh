#!/usr/bin/env bash

OPTIND=1
n=3

# echo "$@"
# echo "$*"

# data_dir=.

# # mkdir -p $data_dir
# if [ "$show_all"]; then
#     echo "ssss"
# fi

# SHOW=$( [ "$n" -eq 1 -o "$show_all" ] && echo "showing" || echo "" )
# if [ "SHOW" ]; then
#     echo "aaaa"
# fi

# echo 2>&1

co=co100

file_name=$co/public.toml

# echo $file_name

GO=/usr/bin/go
$GO run pow.go $file_name