Harvester-installer CI Job Scripts
==================================

Introduction
------------

This directory containers the [Ansible] playbook for harvester-installer CI
job. The harvester-installer CI job is responsible for validating the pull
requests (for the harvester-installer repo) by performing the following:

1. checkout the given pull request
2. build the artifacts (i.e. ISO, ramdisk, kernel, and rootfs) by running the
   `make` command
3. test the newly built artifacts in the [Harvester Vagrant iPXE] environment
4. post the test result to the pull request

Jenkins
-------

The harvester-installer CI job is running on [Jenkins], which is accessible by
either clicking on the `Details` link next to the `Vagrant installation testing`
check job from the pull request, or explicitly via
`https://ci.harvesterhci.io/job/harvester-vagrant-installation-test/<pull request number>/`.
Though anonymouse access is disabled, developers can login as
`harvester/harvester030` to view the job result.

**NOTE:** developers only read access to the job.

Run The Job Locally
-------------------

Though the Ansible playbook is meant to be used by CI, you one can run it
locally as desire. To run it locally, make sure the following variables are
sepcified.

* `WORKSPACE`: this should point to the parent dir of the harvester-installer
   directory.
* `PR_ID`: pull request ID

**WARNING:** the playbook will clone the `harvester/ipxe-examples` repository
under the `WORKSPACE` directory. If you already have `harvester/ipxe-examples'
checked out, make sure to move it out of the way so it won't get overwritten.
Also, make sure your host satisfy the minimal requirements specified in
https://github.com/harvester/ipxe-examples/tree/main/vagrant-pxe-harvester#prerequisites
You can install the latest version of [Ansible] via [Python PIP].

To run the job locally:

1. Checkout the pull request branch.
2. `ansible-playbook -e WORKSPACE=<harvester-installer parent dir> -e PR_ID=<pull request ID> run_vagrant_install_test.yml`


[Ansible]: https://www.ansible.com/
[Harvester Vagrant iPXE]: https://github.com/harvester/ipxe-examples/tree/main/vagrant-pxe-harvester
[Jenkins]: https://www.jenkins.io/
[Python PIP]: https://pip.pypa.io/en/stable/
