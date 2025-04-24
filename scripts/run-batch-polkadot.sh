#!/bin/sh

for chain in polkadot assethub people collectives frequency mythical; do
    ~/src/polkadot/dotidx/dixbatch -conf ~/src/polkadot/dotidx/conf/conf-horn.toml -chain $chain -relayChain polkadot
done
