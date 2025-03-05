#!/bin/bash

mkdir -p logs run

nginx -p `pwd` -c ./etc/nginx/nginx.conf
