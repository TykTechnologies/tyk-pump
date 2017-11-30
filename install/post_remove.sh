#!/bin/bash
echo "Removing init scripts..."

SYSTEMD="/lib/systemd/system"
UPSTART="/etc/init"
SYSV1="/etc/init.d"
SYSV2="/etc/rc.d/init.d/"
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

if [ -f "/lib/systemd/system/tyk-pump.service" ]; then
	echo "Found Systemd"
	echo "Stopping the service"
	systemctl stop tyk-pump.service
	echo "Removing the service"
	rm /lib/systemd/system/tyk-pump.service
	systemctl --system daemon-reload
fi

if [ -f "/etc/init/tyk-pump.conf" ]; then
	echo "Found upstart"
	echo "Stopping the service"
	service tyk-pump stop
	echo "Removing the service"
	rm /etc/init/tyk-pump.conf
fi

if [ -f "/etc/init.d/tyk-pump" ]; then
	echo "Found Sysv1"
	/etc/init.d/tyk-pump stop
	rm /etc/init.d/tyk-pump
fi

if [ -f "/etc/rc.d/init.d/tyk-pump" ]; then
	echo "Found Sysv2"
	echo "Stopping the service"
	/etc/rc.d/init.d/tyk-pump stop
	echo "Removing the service"
	rm /etc/rc.d/init.d/tyk-pump
fi
