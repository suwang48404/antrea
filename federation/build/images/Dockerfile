#builder
FROM golang:1.13 as builder

# install controller-gen
RUN	CONTROLLER_GEN_TMP_DIR=$(mktemp -d) &&\
	cd $CONTROLLER_GEN_TMP_DIR && \
	go mod init tmp  && \
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 &&\
	rm -rf $$CONTROLLER_GEN_TMP_DIR

# install kustomize
RUN	KUSTOMIZE_TMP_DIR=$(mktemp -d) &&\
	cd $KUSTOMIZE_TMP_DIR && \
	go mod init tmp  && \
	go get sigs.k8s.io/kustomize/kustomize/v3@v3.8.5 &&\
	rm -rf $$KUSTOMIZE_TMP_DIR

# install mockgen
RUN	GOMOCK_TMP_DIR=$(mktemp -d) &&\
	cd $GOMOCK_TMP_DIR && \
	go mod init tmp  && \
	go get github.com/golang/mock/mockgen@v1.4.3 &&\
	rm -rf $$GOMOCK_GEN_DIR

# install golangci-lint
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.26.0

ENV PATH=$PATH:$GOPATH/bin

# ovs-tester
FROM antrea/openvswitch:2.14.0 as ovs-tester
RUN apt-get update && \
    apt-get install -y --no-install-recommends iproute2 iptables ipset make wget gcc libc6-dev ca-certificates git iputils-ping conntrack iperf ethtool && \
    rm -rf /var/cache/apt/* /var/lib/apt/lists/*

ENV GO_VERSION=1.13.12
ENV GOPATH /go

RUN wget -q -O - https://golang.org/dl/go$GO_VERSION.linux-amd64.tar.gz | tar xz -C /usr/local/ && \
    export PATH="/usr/local/go/bin:$PATH" && \
    mkdir -p "$GOPATH/src" "$GOPATH/bin" && chmod -R 777 "$GOPATH"

ENV PATH $GOPATH/bin:/usr/local/go/bin/:$PATH
COPY ./bin/ovs-note-decode /usr/bin

# product
# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot as product
WORKDIR /
COPY ./bin/mcs-controller /
USER nonroot:nonroot
