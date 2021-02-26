ARG REPO
ARG TAG
FROM ${REPO}/k3os-tar:${TAG} as tar

ARG REPO
ARG TAG
FROM ${REPO}/k3os-iso:${TAG} as iso

ARG REPO
ARG TAG
FROM ${REPO}/k3os-kernel:${TAG} as kernel

ARG REPO
ARG TAG
FROM ${REPO}/k3os-base:${TAG}
ARG ARCH

COPY --from=kernel /output/vmlinuz /output/k3os-vmlinuz-${ARCH}
COPY --from=kernel /output/initrd /output/k3os-initrd-${ARCH}
COPY --from=iso /output/k3os.iso /output/k3os-${ARCH}.iso
