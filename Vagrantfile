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
    apt-get install -y apt-transport-https ca-certificates curl unzip software-properties-common
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
    add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu bionic stable"
    apt-get update
    apt-get install -y docker-ce

    # Setup go paths
    echo "export GOPATH=/home/vagrant/go" >> /home/vagrant/.bashrc
    echo "export PATH=$PATH:/usr/local/go/bin" >> /home/vagrant/.bashrc
    source /home/vagrant/.bashrc

    # Install golang-1.14.3
    if [ ! -f "/usr/local/go/bin/go" ]; then
      curl -s -L -o go1.14.3.linux-amd64.tar.gz https://dl.google.com/go/go1.14.3.linux-amd64.tar.gz
      sudo tar -C /usr/local -xzf go1.14.3.linux-amd64.tar.gz
      sudo chmod +x /usr/local/go
      rm -f go1.14.3.linux-amd64.tar.gz
    fi

    # Install nomad-1.0.2
    if [ ! -f "/usr/bin/nomad" ]; then
      wget --quiet https://releases.hashicorp.com/nomad/1.0.2/nomad_1.0.2_linux_amd64.zip
      unzip nomad_1.0.2_linux_amd64.zip -d /usr/bin
      chmod +x /usr/bin/nomad
      rm -f nomad_1.0.2_linux_amd64.zip
    fi

    # Run setup
    cd /home/vagrant/go/src/github.com/Roblox/nomad-node-problem-detector/vagrant
    ./setup.sh
  SHELL
end
