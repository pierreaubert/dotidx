#!/bin/sh

/data/dot-node/polkadot-sdk/target/release/polkadot \
    --chain polkadot \
    --name SpinIT \
    --state-pruning archive \
    --blocks-pruning archive \
    --db-cache 16000 \
    --rpc-max-connections 6000 \
    --rpc-rate-limit-whitelisted-ips 192.168.1.0/24 \
    --rpc-rate-limit-trust-proxy-headers \
    --rpc-message-buffer-capacity-per-connection 1024 \
    --rpc-max-batch-request-len 1024 \
    --rpc-cors all \
    --rpc-methods Auto \
    --rpc-external \
    --prometheus-external \
    --allow-private-ip \
    --base-path /data/dot-node/polkadot-node-archive

#    --listen-addr /ip4/192.168.1.37/tcp/9943 \
#    --listen-addr /ip4/192.168.1.37/tcp/9944/ws \
# --enable-offchain-indexing true
#   --wasm-execution Compiled \
