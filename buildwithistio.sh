#!/usr/bin/env bash

set -x
set -e
# Assuming istio is already installed
# Otherwise:
# mkdir -p $GOPATH/src/istio.io/ && \
# cd $GOPATH/src/istio.io/  && \
# git clone https://github.com/istio/istio

MIXER_REPO=$GOPATH/src/istio.io/istio/mixer

ISTIO=$GOPATH/src/istio.io/istio

echo "Coppying resources to local istio repo"
mkdir -p $MIXER_REPO/adapter/vampadapter/config
cp ./adapter/config/config.proto $MIXER_REPO/adapter/vampadapter/config/

echo "Generate adapter definitions"
cd $ISTIO
export REPO_ROOT=${ISTIO}
./bin/mixer_codegen.sh -a mixer/adapter/vampadapter/config/config.proto -x "-s=false -n vampadapter -t logentry"
cd -

echo "Copy adapter config back"
cp $MIXER_REPO/adapter/vampadapter/config/* ./adapter/config/
cp $MIXER_REPO/template/logentry/template.yaml ./adapter/testdata/
cp $MIXER_REPO/adapter/vampadapter/config/vampadapter.yaml ./adapter/testdata/

cp $MIXER_REPO/template/logentry/template.yaml ./adapter/resources/
cp $MIXER_REPO/adapter/vampadapter/config/vampadapter.yaml ./adapter/resources/

echo "Cleanup of local istio repo adapter resources"
rm -rf $MIXER_REPO/adapter/vampadapter

echo "Building ..."
rm vampadapter
CGO_ENABLED=0 go build -o vampadapter
echo "Done"
