#!/bin/sh

PG=/polkadot/postgres_data
mkdir -p ${PG}/logs ${PG}/run
chown -R pierre:pierre ${PG}

# create a PG for dotidx
pg_createcluster -u pierre -g pierre -d ${PG}/dotidx -l ${PG}/logs/dotidx.log -s ${PG}/run -p 5434 16 dotidx
sudo systemctl start postgresql@16-dotidx.service
sudo systemctl status postgresql@16-dotidx.service
sudo systemctl enable postgresql@16-dotidx.service
sudo systemctl status postgresql@16-dotidx.service

# create table
createdb -h ${PG}/run -p 5434 dotidx

# create users
createuser -h ${PG}/run -p 5434 --createdb --createdb --pwprompt dotidx

psql -h ${PG}/run -p 5434 -d dotidx -f etc/postgresql/pg.sql
