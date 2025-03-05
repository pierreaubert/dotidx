#!/bin/sh

pids=$(ps x | grep node | grep substrate-api-sidecar | awk '{print $1}')
if test "$pids" != ""; then
    echo 'Killing' $pids
    kill -9 $pids
else
    echo 'Nothing to do!'
fi;