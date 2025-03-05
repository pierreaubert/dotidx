#!/bin/bash

nginx -p `pwd` -c ./etc/nginx/nginx.conf -s reload

