#!/bin/bash

main() {
    # Install golang-1.17.3
    if [ ! -f "/usr/local/go/bin/go" ]; then
      curl -s -L -o go1.17.3.linux-amd64.tar.gz https://dl.google.com/go/go1.17.3.linux-amd64.tar.gz
      tar -C /usr/local -xzf go1.17.3.linux-amd64.tar.gz
      chmod +x /usr/local/go
      rm -f go1.17.3.linux-amd64.tar.gz
    fi
    echo "golang-1.17.3 installed successfully."

    # Install nomad-1.3.1
    if [ ! -f "/usr/bin/nomad" ]; then
      wget --quiet https://releases.hashicorp.com/nomad/1.3.1/nomad_1.3.1_linux_amd64.zip
      unzip nomad_1.3.1_linux_amd64.zip -d /usr/bin
      chmod +x /usr/bin/nomad
      rm -f nomad_1.3.1_linux_amd64.zip
    fi
    echo "nomad-1.3.1 installed successfully."

    # Install NNPD
    cd ..
    make install
    echo "NNPD installed successfully."

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
