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
FROM --platform=$BUILDPLATFORM golang:1.25.5-alpine3.22 AS builder

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
RUN apk add --no-cache \
        openssl=3.5.4-r0 \
        make=4.4.1-r3 \
        build-base=0.5-r3 \
        lz4-dev=1.10.0-r0 \
        lz4=1.10.0-r0

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
FROM alpine:3.23.2

ENV USER_UID=2001 \
    USER_NAME=appuser \
    GROUP_NAME=appuser

COPY --from=builder --chown=${USER_UID} /build/graphite-remote-adapter /bin/graphite-remote-adapter

# Copy license and notice files
COPY NOTICE /usr/share/doc/graphite-remote-adapter/NOTICE
COPY LICENSE /usr/share/doc/graphite-remote-adapter/LICENSE

# Install runtime dependencies
RUN apk add --no-cache lz4-libs=1.10.0-r0

RUN chmod +x /bin/graphite-remote-adapter \
    && addgroup ${GROUP_NAME} \
    && adduser -D -G ${GROUP_NAME} -u ${USER_UID} ${USER_NAME}

EXPOSE 9092
VOLUME /graphite-remote-adapter
WORKDIR /graphite-remote-adapter

USER ${USER_UID}

ENTRYPOINT ["/bin/graphite-remote-adapter"]
