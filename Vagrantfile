# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure("2") do |config|
  config.vm.box = "ubuntu/bionic64"
  config.vm.box_check_update = false
  config.vm.network "private_network", ip: "192.168.33.2"

  config.vm.provider "virtualbox" do |vb|
    vb.memory = "512"
    vb.cpus = "1"
    vb.name = "vagrant-sandbox"
  end
end
