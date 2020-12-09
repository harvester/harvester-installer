ARG REPO
ARG TAG
FROM ${REPO}/k3os-base:${TAG} AS base
RUN apk --no-cache add \
    grub-bios \
    open-vm-tools
# symlink sh to busybox instead of the default /bin/busybox
# The default breaks when the rootfs is mounted to a directory inside a container
RUN rm /bin/sh && ln -s busybox /bin/sh
