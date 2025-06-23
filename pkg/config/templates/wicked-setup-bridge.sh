#!/bin/sh

ACTION=$1
INTERFACE=$2

case $ACTION in
        pre-up)
                # enable vlan-aware
                ip link set $INTERFACE type bridge vlan_filtering 1
                ;;

        post-up)
                #skip bridge vlan setting when no vlan id or vlan-id=1 specified by user
                if [ {{ .VlanID }} -eq 0 ] || [ {{ .VlanID }} -eq 1 ]; then
                    exit 0
                fi
                #assign user configured vlan,PVID=1 by default
                bridge vlan add vid {{ .VlanID }} dev $INTERFACE self
                bridge vlan add vid {{ .VlanID }} dev {{ .IntfName }}
                ;;
esac
