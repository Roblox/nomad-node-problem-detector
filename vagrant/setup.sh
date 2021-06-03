#!/bin/bash

set -euo pipefail

main() {
    # Install nomad-node-problem-detector (nnpd)
    cd /home/vagrant/go/src/github.com/Roblox/nomad-node-problem-detector
    export PATH=$PATH:/usr/local/go/bin
    make install

    cat << EOF > nomad.service
[Unit]
Description=nomad server + client (dev)
Documentation=https://nomadproject.io
After=network.target
[Service]
ExecStart=/usr/bin/nomad agent -dev -bind=0.0.0.0 -alloc-dir=/tmp/nomad/data
KillMode=process
Delegate=yes
LimitNOFILE=1048576
LimitNPROC=infinity
LimitCORE=infinity
TasksMax=infinity
[Install]
WantedBy=multi-user.target
EOF

    mv nomad.service /lib/systemd/system/nomad.service
    systemctl daemon-reload
    systemctl start nomad
    echo "INFO: Setup finished successfully."
}

main "$@"
