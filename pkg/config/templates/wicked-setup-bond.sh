#!/bin/sh

ACTION=$1
INTERFACE=$2

case $ACTION in
        post-up)
                # inherit MAC address
                ip link set dev {{ .IntfName }} address $(ip -json link show dev $INTERFACE | jq -j '.[0]["address"]')

                #skip bridge vlan setting when no vlan id or vlan id=1 specified by user
                if [ {{ .VlanID }} -eq 0 ] || [ {{ .VlanID }} -eq 1 ]; then
                    exit 0
                fi
                #assign user configured vlan,PVID=1 by default
                bridge vlan add vid {{ .VlanID }} dev $INTERFACE
                ;;

esac
