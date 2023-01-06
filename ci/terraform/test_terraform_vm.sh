#!/bin/bash -ex

if [[ $# != 1 ]]
then
        echo "We need the settings.yaml from ipxe repo"
        exit 1
fi

echo "test vm network..."
TEST_VM_IP=$(./terraform show -json |jq '.values.root_module.resources[] |select (.name=="cirros-01")'|jq .values.network_interface[0].ip_address)
SETTINGS=$1

NODE0_IP=$(yq e ".harvester_network_config.cluster[0].ip" ${SETTINGS})

# sometimes VM network is not ready immediately, try three times and sleep 20 seconds on each try
for i in {1..3}
do
    sleep 20
    SSHKEY=./tmp-ssh-key
    ssh -i ${SSHKEY} rancher@$NODE0_IP "ping -c 5 $TEST_VM_IP"
    cmd_ret=$?
    if [ $cmd_ret == 0 ]; then
        break
    elif [ $i == 3 ]; then
        exit 1
    fi
done