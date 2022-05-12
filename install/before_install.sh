#!/bin/bash

# Generated by: tyk-ci/wf-gen
# Generated on: Tue Dec 28 22:29:08 UTC 2021

# Generation commands:
# ./pr.zsh -base release-1.5 -branch aws/TD-664-r1-5 -title TD-664/Aws-cf-templates -repos tyk-pump
# m4 -E -DxREPO=tyk-pump


echo "Creating user and group..."
GROUPNAME="tyk"
USERNAME="tyk"

getent group "$GROUPNAME" >/dev/null || groupadd -r "$GROUPNAME"
getent passwd "$USERNAME" >/dev/null || useradd -r -g "$GROUPNAME" -M -s /sbin/nologin -c "Tyk service user" "$USERNAME"
