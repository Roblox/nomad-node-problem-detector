#!/bin/bash

set -e

main() {
   tar -xvf /tmp/nnpd.tar -C /alloc >/dev/null 2>&1
}

main "$@"
