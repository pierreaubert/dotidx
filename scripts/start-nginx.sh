#!/bin/bash

mkdir -p logs run

nginx -p `pwd` -c ./etc/nginx/nginx.conf

for i in `seq 1 20`; do
    curl http://192.168.1.37:10800/blocks/2410000${i} | json_pp - > tests/data/blocks/ex-2410000${i}.json
done
