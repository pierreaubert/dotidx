#!/bin/bash

ROOT=`pwd`

mkdir -p logs run

IP="127.0.0.1"

if test "$OS" = "Linux"; then
    IP=$(ip a | grep 192 | cut -d ' ' -f 6 | cut -d '/' -f 1 | head -1)
elif test "$OS" = "Darwin"; then
    ulimit -n 10240
    # IP=$(/sbin/ifconfig| grep 'inet ' | grep broadcast | cut -d ' ' -f 2 | head -1)
fi

SAS_EXPRESS_PORT_START=10800
SAS_SUBSTRATE_TYPES_BUNDLE=undefined
SAS_SUBSTRATE_TYPES_CHAIN=undefined
SAS_SUBSTRATE_TYPES_SPEC=undefined
SAS_SUBSTRATE_TYPES=undefined
SAS_SUBSTRATE_CACHE_CAPACITY=undefined

for p in `seq 1 15`; do
    touch "$ROOT/logs/sidecar-$p.log"
    SAS_LOG_LEVEL="debug" \
    SAS_LOG_JSON=false \
    SAS_LOG_WRITE=true \
    SAS_WRITE_PATH="$ROOT/logs" \
    SAS_SUBSTRATE_URL="ws://127.0.0.1:9944" \
    SAS_EXPRESS_BIND_HOST="$IP" \
    SAS_EXPRESS_KEEP_ALIVE_TIMEOUT=5000 \
    SAS_EXPRESS_MAX_BODY="10mb" \
    SAS_EXPRESS_INJECTED_CONTROLLERS=false \
    SAS_METRICS_ENABLED=false \
    SAS_METRICS_PROM_HOST="192.168.1.32" \
    SAS_METRICS_PROM_PORT=9100 \
    SAS_METRICS_LOKI_HOST="192.168.1.32" \
    SAS_METRICS_LOKI_PORT=3100 \
    SAS_METRICS_INCLUDE_QUERYPARAMS=false \
    SAS_EXPRESS_PORT=$(($SAS_EXPRESS_PORT_START+$p)) \
    "$ROOT/node_modules/.bin/substrate-api-sidecar" 2>&1 >> "$ROOT/logs/sidecar-$p.log" &
done

