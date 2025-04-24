#!/bin/sh

for chain in polkadot assethub people collectives acurast; do
    ~/src/polkadot/dotidx/dixbatch -conf ~/src/polkadot/dotidx/conf/conf-horn.toml -chain $chain -relayChain kusama
done
