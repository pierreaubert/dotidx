#!/bin/bash

mkdir -p logs run conf

OS=$(uname)
IP="127.0.0.1"
HOST=$(hostname)

if test "$OS" = "Linux"; then
    IP=$(ip a | grep 192 | cut -d ' ' -f 6 | cut -d '/' -f 1 | head -1)
elif test "$OS" = "Darwin"; then
    ulimit -n 10240
    # IP=$(/sbin/ifconfig| grep 'inet ' | grep broadcast | cut -d ' ' -f 2 | head -1)
fi

echo "Using: ip=$IP"

sed -e "s/localhost/$IP/g" ./etc/nginx/nginx.conf > conf/nginx.conf

nginx -p `pwd` -c ./conf/nginx.conf

