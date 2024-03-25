module github.com/harvester/harvester-installer

go 1.21

require (
	github.com/harvester/go-common v0.0.0-20230718010724-11313421a8f5
	github.com/imdario/mergo v0.3.12
	github.com/insomniacslk/dhcp v0.0.0-20210827173440-b95caade3eac
	github.com/jroimartin/gocui v0.4.0
	github.com/mudler/yip v0.0.0-20211129144714-088f39125cf7
	github.com/pkg/errors v0.9.1
	github.com/rancher/mapper v0.0.0-20190814232720-058a8b7feb99
	github.com/rancher/wharfie v0.6.5
	github.com/sirupsen/logrus v1.9.2
	github.com/stretchr/testify v1.8.1
	github.com/tredoe/osutil v1.3.6
	github.com/vishvananda/netlink v1.1.0
	golang.org/x/crypto v0.14.0
	golang.org/x/net v0.17.0
	golang.org/x/sys v0.13.0
	gopkg.in/ini.v1 v1.63.2
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/apimachinery v0.25.4
)

require (
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/coreos/yaml v0.0.0-20141224210557-6b16a5714269 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/cli v20.10.20+incompatible // indirect
	github.com/docker/distribution v2.8.2+incompatible // indirect
	github.com/docker/docker v20.10.27+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.7.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/godbus/dbus/v5 v5.0.4 // indirect
	github.com/google/go-containerregistry v0.12.2-0.20230106184643-b063f6aeac72 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/go-multierror v1.1.0 // indirect
	github.com/itchyny/gojq v0.12.2 // indirect
	github.com/itchyny/timefmt-go v0.1.2 // indirect
	github.com/klauspost/compress v1.15.11 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/mattn/go-shellwords v1.0.10 // indirect
	github.com/mdlayher/ethernet v0.0.0-20190606142754-0394541c37b7 // indirect
	github.com/mdlayher/raw v0.0.0-20191009151244-50f2db8cc065 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/nsf/termbox-go v1.1.1 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0-rc2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rancher-sandbox/cloud-init v1.14.3-0.20210913085759-bf90bf5eb77e // indirect
	github.com/rancher/wrangler v0.0.0-20190426050201-5946f0eaed19 // indirect
	github.com/twpayne/go-vfs v1.5.0 // indirect
	github.com/u-root/uio v0.0.0-20210528114334-82958018845c // indirect
	github.com/vishvananda/netns v0.0.0-20210104183010-2eb08e3e575f // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	golang.org/x/sync v0.1.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/utils v0.0.0-20221011040102-427025108f67 // indirect
)

replace (
	github.com/nsf/termbox-go => github.com/Harvester/termbox-go v1.1.1-0.20210318083914-8ab92204a400
	github.com/rancher/wrangler => github.com/rancher/wrangler v1.1.1
	k8s.io/api => k8s.io/api v0.24.10
	k8s.io/apimachinery => k8s.io/apimachinery v0.24.10
	k8s.io/client-go => k8s.io/client-go v0.24.10
)
