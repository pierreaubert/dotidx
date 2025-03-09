#!/bin/sh

PG=/data/dot-node/pg
mkdir -p ${PG}/log ${PG}/run

# create a PG for dotlake
pg_createcluster -u pierre -g pierre -d ${PG}/16/dotlake -l $PG/log -p 5434 16 dotlake
sudo systemctl restart postgresql@16-dotlake.service
sudo systemctl status postgresql@16-dotlake.service

# create table
createdb -h /tmp -p 5434 dotlake

# create users
createuse -h /tmp -p 5434 --createdb --createdb --pwprompt dotlake

psql -h /tmp -p 5434 -f etc/postgresql/pg.sql
