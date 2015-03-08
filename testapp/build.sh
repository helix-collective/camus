#!/bin/bash

npm install

rm -rf build
mkdir build
cp *.js build/
cp -a node_modules build/

echo "Built to `pwd`/build"

