#!/bin/sh

# use 2 disks in this case and stipped them

nvme list

bs0=$(nvme id-ns -H /dev/nvme0n1 | grep LBA | grep bytes | awk '{print $12}')
echo "Block size ${bs0} on disk 0"
bs1=$(nvme id-ns -H /dev/nvme1n1 | grep LBA | grep bytes | awk '{print $12}')
echo "Block size ${bs1} on disk 1"

ns1=$(nvme id-ctrl /dev/nvme0 | grep nn)
echo "Number of namespace ${ns0} on disk 0"
ns1=$(nvme id-ctrl /dev/nvme1 | grep nn)
echo "Number of namespace ${ns1} on disk 1"


VOLUME=dotlake

zpool destroy ${VOLUME}
zpool create -o ashift=9 -o autoexpand=on ${VOLUME} /dev/nvme0n1 /dev/nvme1n1
zfs create ${VOLUME}/data -o mountpoint=/polkadot/postgresql
# zfs create ${VOLUME}/wal-16 -o mountpoint=/polkadot/postgresql/16/polkadot/pg_wal

zfs create ${VOLUME}/ts_slow0 -o mountpoint=/dotlake/slow0
zfs create ${VOLUME}/ts_slow1 -o mountpoint=/dotlake/slow1
zfs create ${VOLUME}/ts_slow2 -o mountpoint=/dotlake/slow2
zfs create ${VOLUME}/ts_slow3 -o mountpoint=/dotlake/slow3

zfs create ${VOLUME}/ts_fast0 -o mountpoint=/dotlake/fast0
zfs create ${VOLUME}/ts_fast1 -o mountpoint=/dotlake/fast1
zfs create ${VOLUME}/ts_fast2 -o mountpoint=/dotlake/fast2
zfs create ${VOLUME}/ts_fast3 -o mountpoint=/dotlake/fast3

zfs set recordsize=16k ${VOLUME}
zfs set compression=zstd-3 ${VOLUME}
zfs set atime=off ${VOLUME}
zfs set xattr=sa ${VOLUME}
zfs set logbias=latency ${VOLUME}
zfs set redundant_metadata=most ${VOLUME}

zpool upgrade ${VOLUME}




