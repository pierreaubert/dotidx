#!/bin/sh
make
if test -x /dotidx/bin/dixfe; then
    mv /dotidx/bin/dixfe /dotidx/bin/dixfe.old;
fi
cp dixfe /dotidx/bin/dixfe
systemctl --user restart dixfe
systemctl --user status dixfe
rsync -arv app/* pierre@192.168.1.18:/var/www/html/dotidx-dev

