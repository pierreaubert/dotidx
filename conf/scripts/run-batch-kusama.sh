#!/bin/sh

DIX=~/src/dotidx

for chain in kusama assethub people collectives acurast; do
    $DIX/bin/dixbatch -conf $DIX/conf/conf-horn.toml -chain $chain -relayChain kusama
done
