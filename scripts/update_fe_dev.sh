#!/bin/sh
make
if test -x /dotidx/bin/dixfe; then
    mv /dotidx/bin/dixfe /dotidx/bin/dixfe.old;
fi
cp dixfe /dotidx/bin/dixfe
systemctl --user restart dixfe
systemctl --user status dixfe
npm run bulma-sass
cp app/dix-large.css app/dix.css
npm run build
rsync -arv --delete app/dist/* pierre@192.168.1.18:/var/www/html/dotidx-dev
rsync -arv --delete app/dist/* pierre@192.168.1.36:/dotidx/static


