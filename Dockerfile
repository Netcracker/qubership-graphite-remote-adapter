# Copyright 2024-2026 NetCracker Technology Corporation
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Build the adapter binary
FROM --platform=$BUILDPLATFORM golang:1.26.6-alpine3.24@sha256:af8d6740070b8906d12eae1c3e3ea0957fb63f492051ea05e354c38ef9fe88df AS builder

ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH
ARG GIT_REVISION
ARG GIT_BRANCH
ARG GOPROXY=""

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# Cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download -x

# Copy the go source
COPY main.go main.go
COPY client/ client/
COPY config/ config/
COPY ui/ ui/
COPY utils/ utils/
COPY web/ web/
COPY VERSION VERSION

# Install LZ4 libraries to build
# renovate: datasource=repology depName=alpine_3_24/openssl versioning=apk
ARG OPENSSL_VERSION=3.5.7-r0
# renovate: datasource=repology depName=alpine_3_24/make versioning=apk
ARG MAKE_VERSION=4.4.1-r4
# renovate: datasource=repology depName=alpine_3_24/build-base versioning=apk
ARG BUILD_BASE_VERSION=0.5-r4
# renovate: datasource=repology depName=alpine_3_24/lz4-dev versioning=apk
ARG LZ4_DEV_VERSION=1.10.0-r1
# renovate: datasource=repology depName=alpine_3_24/lz4 versioning=apk
ARG LZ4_VERSION=1.10.0-r1
RUN apk add --no-cache \
        openssl=${OPENSSL_VERSION} \
        make=${MAKE_VERSION} \
        build-base=${BUILD_BASE_VERSION} \
        lz4-dev=${LZ4_DEV_VERSION} \
        lz4=${LZ4_VERSION}

# Build
RUN CGO_ENABLED=1 CC=gcc GOOS=${TARGETOS} GOARCH=${TARGETARCH} GO111MODULE=on go build \
    -v -o /build/graphite-remote-adapter \
    -gcflags all=-trimpath=${GOPATH} \
    -asmflags all=-trimpath=${GOPATH} \
    -ldflags="-X 'github.com/prometheus/common/version.Version=$(cat VERSION)' \
        -X 'github.com/prometheus/common/version.Revision=${GIT_REVISION}' \
        -X 'github.com/prometheus/common/version.Branch=${GIT_BRANCH}' \
        -X 'github.com/prometheus/common/version.BuildDate=$(date +"%Y%m%d-%H:%M:%S")'" \
    ./

# Use alpine tiny images as a base
FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

ENV USER_UID=2001 \
    USER_NAME=appuser \
    GROUP_NAME=appuser

COPY --from=builder --chown=${USER_UID} /build/graphite-remote-adapter /bin/graphite-remote-adapter

# Copy license and notice files
COPY NOTICE /usr/share/doc/graphite-remote-adapter/NOTICE
COPY LICENSE /usr/share/doc/graphite-remote-adapter/LICENSE

# Install runtime dependencies
# renovate: datasource=repology depName=alpine_3_24/lz4-libs versioning=apk syncWith=alpine
ARG LZ4_LIBS_VERSION=1.10.0-r1
RUN apk add --no-cache lz4-libs=${LZ4_LIBS_VERSION}

RUN chmod +x /bin/graphite-remote-adapter \
    && addgroup ${GROUP_NAME} \
    && adduser -D -G ${GROUP_NAME} -u ${USER_UID} ${USER_NAME}

EXPOSE 9092
VOLUME /graphite-remote-adapter
WORKDIR /graphite-remote-adapter

USER ${USER_UID}

ENTRYPOINT ["/bin/graphite-remote-adapter"]
