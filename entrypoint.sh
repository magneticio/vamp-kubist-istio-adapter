#!/usr/bin/env bash

set -e

kubectl apply -f /app/template.yaml
kubectl apply -f /app/vampadapter.yaml

sed "s/TARGET_NAMESPACE/${VAMP_VIRTUALCLUSTER}/g" "/app/setup.yml" >  "/app/setup-with-target.yml"

kubectl apply -f /app/setup-with-target.yml

./vampadapter
