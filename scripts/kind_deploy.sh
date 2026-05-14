#!/usr/bin/env bash
set -euo pipefail

cluster="${KIND_CLUSTER:-contentflow}"
image="${CONTENTFLOW_IMAGE:-contentflow:local}"
goproxy="${GOPROXY:-https://proxy.golang.org,direct}"

if ! kind get clusters | grep -qx "${cluster}"; then
  kind create cluster --name "${cluster}" --config deployments/k8s/kind/kind-config.yaml
fi

docker build \
  --build-arg GOPROXY="${goproxy}" \
  -f deployments/Dockerfile \
  -t "${image}" \
  .

kind load docker-image "${image}" --name "${cluster}"
kubectl apply -k deployments/k8s/base
kubectl -n contentflow rollout status deployment/contentflow-backend --timeout=120s
