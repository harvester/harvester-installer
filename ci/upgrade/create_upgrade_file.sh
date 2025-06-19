#!/bin/bash -ex

cat <<EOF > new-upgrade.yaml
apiVersion: harvesterhci.io/v1beta1
kind: Upgrade
metadata:
  name: master-head
  namespace: harvester-system
spec:
  logEnabled: true
  version: master-head
EOF
