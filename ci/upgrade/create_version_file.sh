#!/bin/bash -ex

cat <<EOF > new-version.yaml
apiVersion: harvesterhci.io/v1beta1
kind: Version
metadata:
  name: master-head
  namespace: harvester-system
spec:
  isoURL: http://localhost:8000/dist/artifacts/harvester-master-amd64.iso
  minUpgradableVersion: v1.3.1
  releaseDate: 202401231
  tags:
  - dev
  - test
EOF
