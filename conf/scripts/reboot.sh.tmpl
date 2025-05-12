#!/bin/sh

COMMAND=$1

systemctl --user $COMMAND relay-node-archive@polkadot.service
systemctl --user $COMMAND relay-node-archive@kusama.service

systemctl --user $COMMAND chain-node-archive@polkadot-frequency.service
systemctl --user $COMMAND chain-node-archive@polkadot-assethub.service
systemctl --user $COMMAND chain-node-archive@polkadot-mythos.service
systemctl --user $COMMAND chain-node-archive@polkadot-collectives.service
systemctl --user $COMMAND chain-node-archive@polkadot-people.service

systemctl --user $COMMAND chain-node-archive@kusama-kusama.service
systemctl --user $COMMAND chain-node-archive@kusama-acurast.service
systemctl --user $COMMAND chain-node-archive@kusama-assethub.service


for chain in polkadot assethub frequency mythos; do
    for count in 1 2 3 4 5; do
	systemctl --user $COMMAND sidecar@polkadot-$chain-$count.service
    done
done

for chain in people collectives; do
    for count in 1 2; do
	systemctl --user $COMMAND sidecar@polkadot-$chain-$count.service
    done
done

for chain in kusama assethub acurast; do
    for count in 1 2 3 4 5; do
	systemctl --user $COMMAND sidecar@kusama-$chain-$count.service
    done
done

systemctl --user $COMMAND dix-nginx.service
systemctl --user $COMMAND dixlive.service
systemctl --user $COMMAND dixcron.service
systemctl --user $COMMAND dixfe.service



