module github.com/harvester/harvester-installer

go 1.18

require (
	github.com/ghodss/yaml v1.0.0
	github.com/imdario/mergo v0.3.12
	github.com/insomniacslk/dhcp v0.0.0-20210827173440-b95caade3eac
	github.com/jroimartin/gocui v0.4.0
	github.com/mudler/yip v0.0.0-20211129144714-088f39125cf7
	github.com/pkg/errors v0.9.1
	github.com/rancher/mapper v0.0.0-20190814232720-058a8b7feb99
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.8.1
	github.com/tredoe/osutil v1.3.6
	github.com/vishvananda/netlink v1.1.0
	golang.org/x/crypto v0.0.0-20220315160706-3147a52a75dd
	golang.org/x/net v0.7.0
	golang.org/x/sys v0.5.0
	gopkg.in/ini.v1 v1.63.2
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/apimachinery v0.25.4
)

require (
	github.com/coreos/yaml v0.0.0-20141224210557-6b16a5714269 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/go-multierror v1.1.0 // indirect
	github.com/itchyny/gojq v0.12.2 // indirect
	github.com/itchyny/timefmt-go v0.1.2 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/mattn/go-shellwords v1.0.10 // indirect
	github.com/mdlayher/ethernet v0.0.0-20190606142754-0394541c37b7 // indirect
	github.com/mdlayher/raw v0.0.0-20191009151244-50f2db8cc065 // indirect
	github.com/nsf/termbox-go v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rancher-sandbox/cloud-init v1.14.3-0.20210913085759-bf90bf5eb77e // indirect
	github.com/rancher/wrangler v0.0.0-20190426050201-5946f0eaed19 // indirect
	github.com/twpayne/go-vfs v1.5.0 // indirect
	github.com/u-root/uio v0.0.0-20210528114334-82958018845c // indirect
	github.com/vishvananda/netns v0.0.0-20210104183010-2eb08e3e575f // indirect
	golang.org/x/text v0.7.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/nsf/termbox-go => github.com/Harvester/termbox-go v1.1.1-0.20210318083914-8ab92204a400
	github.com/rancher/wrangler => github.com/rancher/wrangler v1.1.0
	golang.org/x/text => golang.org/x/text v0.3.8
	gopkg.in/yaml.v3 => gopkg.in/yaml.v3 v3.0.0-20220521103104-8f96da9f5d5e
	k8s.io/api => github.com/rancher/kubernetes/staging/src/k8s.io/api v1.19.3-k3s1
	k8s.io/apimachinery => github.com/rancher/kubernetes/staging/src/k8s.io/apimachinery v1.19.3-k3s1
	k8s.io/client-go => github.com/rancher/kubernetes/staging/src/k8s.io/client-go v1.19.3-k3s1
)
