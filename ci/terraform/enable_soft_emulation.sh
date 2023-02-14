#!/bin/bash -ex

if [[ $# != 1 ]]
then
        echo "We need the settings.yaml from ipxe repo"
        exit 1
fi

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/" &> /dev/null && pwd )"
IN_SCRIPT_DIR=$SCRIPT_DIR/in-scripts

SETTINGS=$1
SSHKEY="$SCRIPT_DIR/tmp-ssh-key"
NODE0_IP=$(yq e ".harvester_network_config.cluster[0].ip" ${SETTINGS})

ssh-keygen -R ${NODE0_IP} || true

scp -o "StrictHostKeyChecking no" -i ${SSHKEY} $IN_SCRIPT_DIR/patch-kubevirt.sh rancher@$NODE0_IP:/tmp/
ssh -o "StrictHostKeyChecking no" -i ${SSHKEY} rancher@$NODE0_IP sudo -i /tmp/patch-kubevirt.sh
