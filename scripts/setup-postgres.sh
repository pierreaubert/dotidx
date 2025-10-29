#!/bin/sh

PG=/dotidx/db
mkdir -p ${PG}
TS=/dotidx/ts
mkdir -p ${PG}/slow0 ${PG}/slow1 ${PG}/slow2 ${PG}/slow3 ${PG}/slow4 ${PG}/slow5
mkdir -p ${PG}/fast0 ${PG}/fast1 ${PG}/fast2 ${PG}/fast3
sudo chown postgres:postgre ${PG} ${TS}

# create a PG for dotidx
sudo pg_createcluster -d ${PG}/dotidx -p 5434 16 dotidx
sudo systemctl start postgresql@16-dotidx.service
sudo systemctl status postgresql@16-dotidx.service
sudo systemctl enable postgresql@16-dotidx.service
sudo systemctl status postgresql@16-dotidx.service

# create table
sudo su postgres createdb -h ${PG}/run -p 5434 dotidx

# create users
sudo su postgres createuser -h ${PG}/run -p 5434 --createdb --createdb --pwprompt dotidx

sudo su postgres psql -h ${PG}/run -p 5434 -d dotidx -f etc/postgresql/pg.sql
