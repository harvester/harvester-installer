ARG BASE_OS_IMAGE
FROM ${BASE_OS_IMAGE}

RUN curl -sfL https://github.com/rancher/wharfie/releases/latest/download/wharfie-amd64  -o /usr/bin/wharfie && chmod +x /usr/bin/wharfie

# rancherd dev version
RUN curl -L https://github.com/bk201/rancherd/releases/download/dev-20210707/rancherd > /usr/bin/rancherd && \
    chmod +x /usr/bin/rancherd

ARG CACHEBUST
# packages
RUN zypper ref
RUN zypper in -y zstd

COPY files/ /
RUN chmod 0600 /system/oem/*

RUN mkinitrd
