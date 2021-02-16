#!/bin/bash

set -euo pipefail

main() {
  image=$1
  root_dir=$2
  mkdir -p var/lib/nnpd
  cp -R $root_dir/* var/lib/nnpd
  tar -zcf nnpd.tar var/lib/nnpd
  docker build -t $image .
  docker push $image
}

cleanup() {
  rm nnpd.tar >/dev/null 2>&1
  rm -rf var/lib/nnpd >/dev/null 2>&1
}

trap cleanup EXIT

main "$@"
