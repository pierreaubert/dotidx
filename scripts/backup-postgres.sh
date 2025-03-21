#!/bin/sh

export PG=/polkadot/postgres_data

for year in 2020 2021 2022 2023 2024 2025; do
    for month in 01 02 03 04 05 06 07 08 09 10 11 12; do
	for chain in polkadot assethub people collectives; do
	    for table in blocks address2blocks; do
		tablename="chain.blocks_polkadot_${chain}_${year}_${month}"
		dumpfile=${tablename}.dump
		if test ! -f "${dumpfile}"; then
		    echo "Dumping to $dumpfile"
		    # no need for compression which is done by zfs
		    pg_dump -h ${PG}/run -p 5434 -n chain -t "${tablename}" dotidx > "${dumpfile}"
      	        fi
	    done
	done
    done
done
