#!/bin/sh

PG=/polkadot/postgres_data
for year in 2020 2021 2022 2023 2024 2025; do
    for month in 01 02 03 04 05 06 07 08 09 10 11 12; do
        echo "Dumping ${year}_${month}"
        table="chain.blocks_polkadot_polkadot_${year}_${month}"
        pg_dump -h ${PG}/run -p 5434 -n chain -Z -t "${table}" dotidx > "${table}.dump.gz"
    done
done
