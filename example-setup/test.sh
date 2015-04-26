#!/bin/bash

cd `dirname $0`

if [ ! -d deps ]; then
  ./dl-deps.sh
fi

docker build -t camus-test . && docker run camus-test
