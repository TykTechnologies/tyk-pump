#!/bin/bash
echo "Installing init scripts..."

SYSTEMD="/lib/systemd/system"
UPSTART="/etc/init"
SYSV1="/etc/init.d"
SYSV2="/etc/rc.d/init.d/"
DIR="/opt/tyk-pump/install"

if [ -d "$SYSTEMD" ]; then
	echo "Found Systemd"
	cp $DIR/inits/systemd/system/tyk-pump.service /lib/systemd/system/tyk-pump.service
fi

if [ -d "$UPSTART" ]; then
	echo "Found upstart"
	cp $DIR/inits/upstart/conf/tyk-pump.conf /etc/init/
fi

if [ -d "$SYSV1" ]; then
	echo "Found SysV1"
	cp $DIR/inits/sysv/etc/default/tyk-pump /etc/default/tyk-pump
	cp $DIR/inits/sysv/etc/init.d/tyk-pump /etc/init.d/tyk-pump
  	
fi

if [ -d "$SYSV2" ]; then
	echo "Found Sysv2"
  	cp $DIR/inits/sysv/etc/default/tyk-pump /etc/default/tyk-pump
	cp $DIR/inits/sysv/etc/init.d/tyk-pump /etc/rc.d/init.d/tyk-pump
fi