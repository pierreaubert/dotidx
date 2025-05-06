#!/bin/sh
# https://github.com/plotly/plotly.js/blob/master/CUSTOM_BUNDLE.md

VERSION=v3.0.1

cd ~/src
if test ! -d plotly.js; then
    git clone --branch $VERSION https://github.com/plotly/plotly.js.git
fi

cd plotly.js
# this one is working for example
nvm use v20.19.0
npm install
# included only what we need for now
npm run custom-bundle -- --traces scatter,bar --strict --out dotidx

mkdir -p ~/src/dotidx/app/3rdparties
cp dist/plotly-dotidx.min.js ~/src/dotidx/app/3rdparties


