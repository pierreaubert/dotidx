#!/bin/bash

mkdir -p tests/data/blocks

for i in `seq 0 9`; do
    curl http://192.168.1.36:10800/blocks/2410000${i} | json_pp - > tests/data/blocks/ex-2410000${i}.json
done

for i in `seq 0 9`; do
    curl http://192.168.1.36:10800/blocks/2010000${i} | json_pp - > tests/data/blocks/ex-2010000${i}.json
done

for i in `seq 0 9`; do
    curl http://192.168.1.36:10800/blocks/1910000${i} | json_pp - > tests/data/blocks/ex-1910000${i}.json
done

for i in `seq 0 9`; do
    curl http://192.168.1.36:10800/blocks/1210000${i} | json_pp - > tests/data/blocks/ex-1210000${i}.json
done

for i in `seq 0 9`; do
    curl http://192.168.1.36:10800/blocks/410000${i} | json_pp - > tests/data/blocks/ex-410000${i}.json
done

for i in `seq 0 9`; do
    curl http://192.168.1.36:10800/blocks/210000${i} | json_pp - > tests/data/blocks/ex-210000${i}.json
done

for i in `seq 0 9`; do
    curl http://192.168.1.36:10800/blocks/10000${i} | json_pp - > tests/data/blocks/ex-10000${i}.json
done

curl http://192.168.1.36:10800/blocks/24731329 | json_pp - > tests/data/blocks/ex-24731329.json
