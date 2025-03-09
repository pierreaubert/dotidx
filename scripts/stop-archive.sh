#/bin/sh
pid=$(ps auxw | grep release/polkadot | grep -v grep | awk '{print $2}')
kill -9 $pid
