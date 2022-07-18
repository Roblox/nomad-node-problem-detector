# Specify minimum Vagrant version and Vagrant API version
Vagrant.require_version ">= 1.6.0"
VAGRANTFILE_API_VERSION = "2"

# Create box
Vagrant.configure("2") do |config|
  config.vm.define "nnpd"
  config.vm.box = "hashicorp/bionic64"
  config.vm.synced_folder ".", "/home/vagrant/go/src/github.com/Roblox/nomad-node-problem-detector"
  config.ssh.extra_args = ["-t", "cd /home/vagrant/go/src/github.com/Roblox/nomad-node-problem-detector; bash --login"]
  config.vm.network "forwarded_port", guest: 4646, host: 4647, host_ip: "127.0.0.1"
  config.vm.provider "virtualbox" do |vb|
      vb.name = "nnpd"
      vb.cpus = 2
      vb.memory = 2048
  end
  config.vm.provision "shell", inline: <<-SHELL
    # Install and start docker
    apt-get update
    apt-get install -y apt-transport-https ca-certificates curl jq build-essential unzip software-properties-common
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
    add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu bionic stable"
    apt-get update
    apt-get install -y docker-ce

    # Setup go paths
    echo "export GOPATH=/home/vagrant/go" >> /home/vagrant/.bashrc
    echo "export PATH=$PATH:/usr/local/go/bin" >> /home/vagrant/.bashrc
    source /home/vagrant/.bashrc

    # Install golang-1.18.4
    if [ ! -f "/usr/local/go/bin/go" ]; then
      GO_ARCHIVE=go1.18.4.linux-amd64.tar.gz
      curl -s -L -o "${GO_ARCHIVE}" "https://dl.google.com/go/${GO_ARCHIVE}"
      sudo tar -C /usr/local -xzf "${GO_ARCHIVE}"
      sudo chmod +x /usr/local/go
      rm -f "${GO_ARCHIVE}"
    fi

    # Install nomad-1.1.4
    if [ ! -f "/usr/bin/nomad" ]; then
      NOMAD_VERSION=1.1.4
      wget --quiet "https://releases.hashicorp.com/nomad/${NOMAD_VERSION}/nomad_${NOMAD_VERSION}_linux_amd64.zip"
      unzip "nomad_${NOMAD_VERSION}_linux_amd64.zip" -d /usr/bin
      chmod +x /usr/bin/nomad
      rm -f "nomad_${NOMAD_VERSION}_linux_amd64.zip"
    fi

    # Run setup
    chown -R 1000:1000 /home/vagrant/go
    cd /home/vagrant/go/src/github.com/Roblox/nomad-node-problem-detector/vagrant
    ./setup.sh
  SHELL
end
