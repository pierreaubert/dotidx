#!/bin/sh

DIX=~/src/dotidx

for chain in polkadot assethub people collectives frequency mythos; do
    $DIX/bin/dixbatch -conf $DIX/conf/conf-horn.toml -chain $chain -relayChain polkadot
done
