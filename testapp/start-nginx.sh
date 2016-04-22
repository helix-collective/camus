#!/bin/bash

set -euo pipefail

NGINX="$1"
if [ ! "$NGINX" ]; then
  echo "nginx path not provided"
  exit 1
fi

DOCROOT=$2
PORT="$3"
FORWARD_PORT="$4"

cat nginx.conf.tpl | sed "s|%ROOT%|$DOCROOT|" | sed "s/%PORT%/$PORT/" | sed "s/%FORWARD_PORT%/$FORWARD_PORT/" > nginx.conf

exec "$NGINX" -p . -g 'error_log /dev/null;' -c nginx.conf
