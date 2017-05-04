# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure("2") do |config|

  config.vm.define "ubuntu" do |ubuntu|
    ubuntu.vm.box = "wholebits/ubuntu16.04-64"

    ubuntu.vm.network "forwarded_port", guest: 8010, host: 8012, host_ip: "localhost", auto_correct: true
    ubuntu.vm.network "forwarded_port", guest: 6379, host: 6381, host_ip: "localhost", auto_correct: true
    ubuntu.vm.network "forwarded_port", guest: 27017, host: 27019, host_ip: "localhost", auto_correct: true

    ubuntu.vm.provision "ansible", playbook: "packaging/vagrant/playbook.yml"
    ubuntu.vm.network "private_network", ip: "192.92.2.1"
  end

  config.vm.define "centos" do |centos|
    centos.vm.box = "wholebits/centos7"

    centos.vm.network "forwarded_port", guest: 8010, host: 8011, host_ip: "localhost", auto_correct: true
    centos.vm.network "forwarded_port", guest: 6379, host: 6380, host_ip: "localhost", auto_correct: true
    centos.vm.network "forwarded_port", guest: 27017, host: 27018, host_ip: "localhost", auto_correct: true
    centos.vm.provision "ansible", playbook: "packaging/vagrant/playbook.yml"
    centos.vm.network "private_network", ip: "192.92.2.2"
  end

  config.vm.synced_folder '.', '/vagrant/src/github.com/pearsonappeng/tensor', nfs: true
end