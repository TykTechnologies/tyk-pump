#!/bin/sh -x

curl -s https://packagecloud.io/install/repositories/tyk/tyk-pump/script.rpm.sh | sudo bash
sudo yum install -y pygpgme yum-utils wget

sudo rpm -ivh /tmp/tyk-pump.rpm || exit 1

sudo rm -f /home/ec2-user/.ssh/authorized_keys
sudo rm -f /root/.ssh/authorized_keys
