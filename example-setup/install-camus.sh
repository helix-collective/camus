set -ex

if [ `whoami` != root ]; then
   echo "Must run as root"
   exit 1
fi

U=$1

if [ ! "$U" ]; then
  echo "Require user to setup: $U"
  exit 1
fi

UHOME=/home/$U
UGO=$UHOME/goroot

if [ ! -d $UHOME ]; then
  echo "Need $UHOME to exist";
  exit 1
fi

if [ ! -d /usr/local/go/bin ]; then
  echo "go not found. Need to run install-deps.sh"
  exit 1
fi


mkdir $UGO

chown $U:$U $UGO

echo 'export PATH=$PATH:/usr/local/go/bin:'$UGO'/bin' >> $UHOME/.profile
echo 'export GOPATH='$UGO >> $UHOME/.profile

sudo -i -u $U go get github.com/helix-collective/camus
sudo -i -u $U go install github.com/helix-collective/camus

mkdir -p $UHOME/camus
echo '{}' > $UHOME/camus/config.json
chown -R $U:$U $UHOME/camus

echo '#!/bin/bash

set -ex

camus -server -serverRoot camus' > $UHOME/start-camus.sh

chown -R $U:$U $UHOME/start-camus.sh
chmod a+x $UHOME/start-camus.sh

echo "Now go become user $U and run this: 

screen -S start-camus.sh
"


