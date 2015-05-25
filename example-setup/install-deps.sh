#!/bin/bash

set -ex

# This is in the docker file, but can be run manually
#apt-get install -y wget git build-essential libssl-dev

cd /tmp

# These are in place via the dockerfile, but if you're using
# this script directly, you can uncomment
#wget https://storage.googleapis.com/golang/go1.4.2.linux-amd64.tar.gz
#wget http://www.haproxy.org/download/1.5/src/haproxy-1.5.3.tar.gz

rm -rf /usr/local/go
tar -C /usr/local -xf go1.4.2.linux-amd64.tar.gz

tar xf haproxy-1.5.3.tar.gz
cd haproxy-1.5.3
make TARGET=linux2628 USE_OPENSSL=1
make install
