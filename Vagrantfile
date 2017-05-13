# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure("2") do |config|

  config.vm.define "ubuntu" do |ubuntu|
    ubuntu.vm.box = "wholebits/ubuntu16.04-64"
    ubuntu.vm.provision "ansible", playbook: "packaging/vagrant/playbook.yml"
    ubuntu.vm.network "private_network", ip: "192.92.2.2"
  end

  config.vm.define "centos" do |centos|
    centos.vm.box = "wholebits/centos7"
    centos.vm.provision "ansible", playbook: "packaging/vagrant/playbook.yml"
    centos.vm.network "private_network", ip: "192.92.2.3"
  end

  config.vm.synced_folder '.', '/vagrant/src/github.com/pearsonappeng/tensor', nfs: true
end