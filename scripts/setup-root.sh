#!/bin/sh

# for the node

# for the loadbalancer
apt install nginx

# for sidecar
apt install npm

# for testing
apt install curl

# for database
apt install postgresql-16 postgresql-16-jsquery

# for monitoring
apt install prometheus grafana prometheus-postgres-exporter prometheus-sql-exporter prometheus-node-exporter sensors
