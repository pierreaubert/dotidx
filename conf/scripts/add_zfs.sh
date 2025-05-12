#!/bin/sh

# add a zfs partition for a relay or a chain
# ex usage
#
# sudo add_zfs.sh tank kusama-assethub-node-archive
#
# where tank is a ZFS pool

VOLUME=$1
CHAIN=$2

command=$(zpool status -x $VOLUME)
status=$?
if [ $status -ne 0 ]; then
    echo "$VOLUME does not look like a valid ZFS pool"
    exit 1
fi

DATASET=${VOLUME}/${CHAIN}

command=$(zfs list $DATASET)
status=$?
if [ $status -eq 0 ]; then
    echo "$DATASET already exist!"
    exit 1
fi

zfs create ${DATASET}

zfs set recordsize=16k ${DATASET}
zfs set compression=off ${DATASET}
zfs set atime=off ${DATASET}
zfs set xattr=sa ${DATASET}
zfs set logbias=latency ${DATASET}
zfs set redundant_metadata=most ${DATASET}

zfs list $DATASET
