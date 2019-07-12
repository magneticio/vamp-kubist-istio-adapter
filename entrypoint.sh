#!/usr/bin/env bash

set -e

sed "s/TARGET_NAMESPACE/${VAMP_VIRTUALCLUSTER}/g" "/app/setup.yml" >  "/app/setup-with-target.yml"

kubectl apply -f /app/setup-with-target.yml

./vampadapter
