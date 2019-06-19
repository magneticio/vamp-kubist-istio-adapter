#!/usr/bin/env bash

# Assuming istio is already installed
# Otherwise:
# mkdir -p $GOPATH/src/istio.io/ && \
# cd $GOPATH/src/istio.io/  && \
# git clone https://github.com/istio/istio

MIXER_REPO=$GOPATH/src/istio.io/istio/mixer

ISTIO=$GOPATH/src/istio.io

echo "Coppying resources to local istio repo"
mkdir -p $MIXER_REPO/adapter/vamp-kubist-istio-adapter/config
cp ./adapter/config/config.proto $MIXER_REPO/adapter/vamp-kubist-istio-adapter/config/
cp ./adapter/vampadapter.go $MIXER_REPO/adapter/vamp-kubist-istio-adapter/

echo "Generate adapter definitions"
cd $MIXER_REPO/adapter/vamp-kubist-istio-adapter
go generate ./...
go build ./...
cd -

echo "Copy adapter config back"
cp $MIXER_REPO/adapter/vamp-kubist-istio-adapter/config/* ./adapter/config/
cp $MIXER_REPO/adapter/vamp-kubist-istio-adapter/config/vampadapter.yaml ./adapter/testdata/

echo "Cleanup of local istio repo adapter resources"
rm -rf $MIXER_REPO/adapter/vamp-kubist-istio-adapter

echo "Building ..."
rm vampadapter
CGO_ENABLED=0 go build -o vampadapter
