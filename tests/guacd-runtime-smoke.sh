#!/bin/sh
set -eu
# Run against the official image, without network or remote credentials.
# Numeric UID alone is insufficient: FreeRDP needs a passwd entry and home.
docker run --rm --network none --entrypoint sh "${1:-guacamole/guacd:1.6.0}" -ec '
  test "$(id -u)" = 1000
  account=$(getent passwd 1000)
  runtime_home=$(echo "$account" | cut -d: -f6)
  test "$runtime_home" = /home/guacd
  test -d "$runtime_home"
  test -w "$runtime_home"
  echo "PASS: official non-root UID, passwd identity and writable FreeRDP home"
'
