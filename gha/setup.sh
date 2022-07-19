#!/bin/bash

main() {
    # Install golang
    if [ ! -f "/usr/local/go/bin/go" ]; then
      GO_ARCHIVE=go1.18.4.linux-amd64.tar.gz
      curl -s -L -o "${GO_ARCHIVE}" "https://dl.google.com/go/${GO_ARCHIVE}"
      tar -C /usr/local -xzf "${GO_ARCHIVE}"
      chmod +x /usr/local/go
      rm -f "${GO_ARCHIVE}"
    fi
    go_version=$(/usr/local/go/bin/go version)
    echo "golang ${go_version} installed successfully."

    # Install nomad
    if [ ! -f "/usr/bin/nomad" ]; then
      NOMAD_VERSION=1.1.4
      wget --quiet "https://releases.hashicorp.com/nomad/${NOMAD_VERSION}/nomad_${NOMAD_VERSION}_linux_amd64.zip"
      unzip "nomad_${NOMAD_VERSION}_linux_amd64.zip" -d /usr/bin
      chmod +x /usr/bin/nomad
      rm -f "nomad_${NOMAD_VERSION}_linux_amd64.zip"
    fi
    echo "nomad-1.1.0 installed successfully."

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
