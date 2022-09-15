#!/bin/bash

HARV_CONFIG="${HARV_CONFIG:-/oem/99_custom.yaml}"
HARV_MGMT="harvester-mgmt"
IFCFG=
IFROUTE=

function migrate_mgmt_config () {
	MODE=
	# search all files
	for (( i=0; ; i++ ))
	do
		if ! yq -e ".stages.initramfs[0].files[$i]" "$HARV_CONFIG" > /dev/null 2>&1; then
			echo "end files"
			break
		fi

		if yq ".stages.initramfs[0].files[$i].path" "$HARV_CONFIG" | grep -q "/etc/sysconfig/network/ifcfg-${HARV_MGMT}"; then
			# save config
			IFCFG=$(yq ".stages.initramfs[0].files[$i].content" "$HARV_CONFIG")
			echo "$IFCFG"
		fi

		if yq ".stages.initramfs[0].files[$i].path" "$HARV_CONFIG" | grep -q "/etc/sysconfig/network/ifroute-${HARV_MGMT}"; then
			# save config
			IFROUTE=$(yq ".stages.initramfs[0].files[$i].content" "$HARV_CONFIG")
			echo "$IFROUTE"
		fi
	done

	# check DHCP or static
	if echo "$IFCFG" | grep -q "BOOTPROTO='static'"; then
		MODE="static"
	elif echo "$IFCFG" | grep -q "BOOTPROTO='dhcp'"; then
		MODE="dhcp"
	else
		echo "error detect bootproto mode"
		exit 1
	fi

	# start patch
	# remove all ifcfg and ifroute
	if [ $MODE == "dhcp" ]; then
		yq -i 'del( .stages.initramfs[0].commands[] | select(. == "rm -f /etc/sysconfig/network/ifroute-harvester-mgmt"))' "$HARV_CONFIG"
	fi
	yq -i 'del( .stages.initramfs[0].files[] | select(.path == "*ifcfg*"))' "$HARV_CONFIG"
	yq -i 'del( .stages.initramfs[0].files[] | select(.path == "*ifroute*"))' "$HARV_CONFIG"
	sed -i "s/iface: $HARV_MGMT/iface: \"\"/g" "$HARV_CONFIG"

	# add file
	if [ $MODE == "dhcp" ]; then
		yq -i '.stages.initramfs[0].commands += "rm -f /etc/sysconfig/network/ifroute-mgmt-br"' "$HARV_CONFIG"
	fi

	cat <<EOF | yq eval-all -i 'select(fileIndex==0).stages.initramfs[0].files += select(fileIndex==1) | select(fileIndex==0)' "$HARV_CONFIG" -
- path: /etc/wicked/scripts/setup_bond.sh
  permissions: 755
  owner: 0
  group: 0
  content: |+
    #!/bin/sh

    ACTION=\$1
    INTERFACE=\$2

    case \$ACTION in
    	post-up)
    		# accept all vlan, PVID=1 by default
    		bridge vlan add vid 2-4094 dev \$INTERFACE
    		;;
    esac
  encoding: ""
  ownerstring: ""

- path: /etc/wicked/scripts/setup_bridge.sh
  permissions: 755
  owner: 0
  group: 0
  content: |+
    #!/bin/sh

    ACTION=\$1
    INTERFACE=\$2

    case \$ACTION in
    	pre-up)
    		# enable vlan-aware
    		ip link set \$INTERFACE type bridge vlan_filtering 1
    		;;

    	post-up)
    		# inherit MAC address from mgmt-bo
    		ip link set dev \$INTERFACE address \$(ip -json link show dev mgmt-bo | jq -j '.[0]["address"]')

    		# accept all vlan, PVID=1 by default
    		bridge vlan add vid 2-4094 dev \$INTERFACE self
    		bridge vlan add vid 2-4094 dev mgmt-bo
    		;;
    esac
  encoding: ""
  ownerstring: ""

- path: /etc/sysconfig/network/ifcfg-mgmt-br
  permissions: 384
  owner: 0
  group: 0
  content: |+
    STARTMODE='onboot'
    BOOTPROTO='${MODE}'
    BRIDGE='yes'
    BRIDGE_STP='off'
    BRIDGE_FORWARDDELAY='0'
    BRIDGE_PORTS='mgmt-bo'
    PRE_UP_SCRIPT="wicked:setup_bridge.sh"
    POST_UP_SCRIPT="wicked:setup_bridge.sh"

    $(echo "$IFCFG" | grep "IPADDR")
    $(echo "$IFCFG" | grep "NETMASK")
    $(echo "$IFCFG" | grep "DHCLIENT_SET_DEFAULT_ROUTE")
    $(echo "$IFCFG" | grep "MTU")
  encoding: ""
  ownerstring: ""

- path: /etc/sysconfig/network/ifcfg-mgmt-bo
  permissions: 384
  owner: 0
  group: 0
  content: |+
    STARTMODE='onboot'
    BONDING_MASTER='yes'
    BOOTPROTO='none'
    POST_UP_SCRIPT="wicked:setup_bond.sh"
    $(echo "$IFCFG" | grep "BONDING_SLAVE_")
    $(echo "$IFCFG" | grep "BONDING_MODULE_OPTS")

    $(echo "$IFCFG" | grep "MTU")
  encoding: ""
  ownerstring: ""
EOF

	if [ $MODE == "static" ]; then
		cat <<EOF | yq eval-all -i 'select(fileIndex==0).stages.initramfs[0].files += select(fileIndex==1) | select(fileIndex==0)' "$HARV_CONFIG" -
- path: /etc/sysconfig/network/ifroute-mgmt-br
  permissions: 384
  owner: 0
  group: 0
  content: |
    $(echo "$IFROUTE" | sed "s/$HARV_MGMT/mgmt-br/g")
  encoding: ""
  ownerstring: ""
EOF
	fi
}

if ! grep -q "$HARV_MGMT" "$HARV_CONFIG"; then
	echo "$HARV_MGMT not found. Skipping."
	exit 0
fi

echo "$HARV_MGMT found."

# backup config with timestamp
TIMESTAMP=$(date "+%Y%m%d-%H%M%S")
cp "$HARV_CONFIG" "${HARV_CONFIG}.${TIMESTAMP}"

migrate_mgmt_config

