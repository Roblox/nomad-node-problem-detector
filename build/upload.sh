#!/bin/bash

set -euo pipefail

main() {
  image=$1
  root_dir=$2

  # Note: This only removes var (temp dir) from the current build directory
  # (nomad-node-problem-detector/build) and not /var
  if [ -d "var" ]; then
     rm -rf var
  fi
  mkdir -p var/lib/nnpd
  cp -R $root_dir/* var/lib/nnpd
  tar -zcf nnpd.tar var/lib/nnpd
  docker build -t $image . >/dev/null >&1
  docker push $image >/dev/null >&1
}

cleanup() {
  if [ -f "nnpd.tar" ]; then
     rm nnpd.tar
  fi
  # Note: This only removes var (temp dir) from the current build directory
  # (nomad-node-problem-detector/build) and not /var
  if [ -d "var" ]; then
     rm -rf var
  fi
}

trap cleanup EXIT

main "$@"
