# Copyright 2024-2025 NetCracker Technology Corporation
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
FROM golang:1.24-alpine3.21 AS builder

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

RUN ls -la /workspace

# Install LZ4 libraries to build
RUN apk add --no-cache \
        openssl=3.3.3-r0 \
        make=4.4.1-r2 \
        build-base=0.5-r3 \
        lz4-dev=1.10.0-r0 \
        lz4=1.10.0-r0

# Build
RUN CGO_ENABLED=1 CC=gcc GOOS=linux GOARCH=amd64 GO111MODULE=on go build \
    -v -o /build/graphite-remote-adapter \
    -gcflags all=-trimpath=${GOPATH} \
    -asmflags all=-trimpath=${GOPATH} \
    ./

# Use alpine tiny images as a base
FROM alpine:3.21.3

ENV USER_UID=2001 \
    USER_NAME=appuser \
    GROUP_NAME=appuser

COPY --from=builder --chown=${USER_UID} /build/graphite-remote-adapter /bin/graphite-remote-adapter
EXPOSE 9092
VOLUME "/graphite-remote-adapter"

RUN chmod +x /bin/graphite-remote-adapter \
    && addgroup ${GROUP_NAME} \
    && adduser -D -G ${GROUP_NAME} -u ${USER_UID} ${USER_NAME}

RUN apk add --no-cache lz4-libs=1.10.0-r0

WORKDIR /graphite-remote-adapter

USER ${USER_UID}
