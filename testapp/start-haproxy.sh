#!/bin/sh

cd `dirname $0`

FRONT="$1"
APP="$2"

if [ $(uname) = "Darwin" ]; then
  TEMP=`mktemp /tmp/haproxy-config.$$`
else
  TEMP=`mktemp`
fi
echo "storing config in $TEMP"

cat haproxy.cfg.tpl | sed "s|%FRONTPORT%|$FRONT|g" | sed "s|%APPPORT%|$APP|g" > "$TEMP"


function finish {
  echo "cleaning up app pid $PID"
  kill $PID
  rm -f PID_FILE
}

trap finish EXIT

#make sure we aren't running in the deploy dir
cd /tmp
exec haproxy -f "$TEMP" &
PID=$!
echo "exec'd $PID"
cd -
echo "$PID" > PID_FILE
echo "wait for $PID"
wait $PID
