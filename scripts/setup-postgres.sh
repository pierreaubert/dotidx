#!/bin/sh

PG=/polkadot/postgres_data
mkdir -p ${PG}/logs ${PG}/run

# create a PG for dotlake
pg_createcluster -u pierre -g pierre -d ${PG}/dotlake -l ${PG}/logs/dotlake.log -s ${PG}/run -p 5434 16 dotlake
sudo systemctl restart postgresql@16-dotlake.service
sudo systemctl status postgresql@16-dotlake.service

# create table
createdb -h ${PG}/run -p 5434 dotlake

# create users
createuse -h ${PG}/run -p 5434 --createdb --createdb --pwprompt dotlake

psql -h ${PG}/run -p 5434 -d dotlake -f etc/postgresql/pg.sql
