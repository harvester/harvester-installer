FROM rancher/hardened-build-base:v1.19.0b1 AS build

ARG DAPPER_HOST_ARCH
ENV ARCH $DAPPER_HOST_ARCH
RUN set -x \
    && apk --no-cache add \
    bash \
    curl \
    file \
    git \
    libseccomp-dev \
    rsync \
    gcc \
    bsd-compat-headers \
    py-pip \
    pigz \
    tar \
    yq


RUN mkdir -p /charts && cd /charts && curl -O https://raw.githubusercontent.com/rancher/rke2/master/charts/build-chart.sh  && chmod +x build-chart.sh
RUN CHART_VERSION="v3.23.3-build2022081002"   CHART_FILE=/charts/rke2-canal.yaml          CHART_BOOTSTRAP=true   /charts/build-chart.sh

FROM rancher/rke2-runtime:v1.24.6-rke2r1
COPY --from=build /charts/rke2-canal.yaml /charts/
