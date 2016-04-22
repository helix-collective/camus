#!/bin/bash

npm install

rm -rf build
mkdir build
cp *.js build/
cp -r data build/
cp deploy.json build/
cp -a node_modules build/
cp haproxy.cfg.tpl build/
cp start-haproxy.sh build/
cp nginx.conf.tpl build/
cp start-nginx.sh build/

echo "Built to `pwd`/build"

