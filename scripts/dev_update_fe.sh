#!/bin/sh
make
# update fe
if test -x /dotidx/bin/dixfe; then
    mv /dotidx/bin/dixfe /dotidx/bin/dixfe.old;
fi
cp ./bin/dixfe /dotidx/bin/dixfe
systemctl --user restart dixfe
systemctl --user status dixfe
# css and compression
npm run bulma-sass
cp app/dix-large.css app/dix.css
npm run build
# web server
rsync -arv --delete app/dist/* pierre@192.168.1.18:/var/www/html/dotidx-dev
# fe server
rsync -arv --delete app/dist/* pierre@192.168.1.36:/dotidx/static
# done
