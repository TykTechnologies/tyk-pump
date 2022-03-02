#!/bin/sh



# Generated by: tyk-ci/wf-gen
# Generated on: Wednesday 02 March 2022 07:39:16 AM UTC

# Generation commands:
# ./pr.zsh -title Sync from latest releng templates -branch test/sync-changes -base master -repos tyk-pump -p
# m4 -E -DxREPO=tyk-pump


if command -V systemctl >/dev/null 2>&1; then
    if [ ! -f /lib/systemd/system/tyk-pump.service ]; then
        cp /opt/tyk-pump/install/inits/systemd/system/tyk-pump.service /lib/systemd/system/tyk-pump.service
    fi
else
    if [ ! -f /etc/init.d/tyk-pump ]; then
        cp /opt/tyk-pump/install/inits/sysv/init.d/tyk-pump /etc/init.d/tyk-pump
    fi
fi
