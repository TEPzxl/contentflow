#!/usr/bin/env bash
set -euo pipefail

base="deployments/k8s/base"
rendered="/tmp/contentflow-k8s.yaml"

test -f "${base}/kustomization.yaml"
test -f "${base}/backend-deployment.yaml"
test -f "${base}/frontend-deployment.yaml"
test -f "${base}/worker-deployment.yaml"
test -f "${base}/scheduler-deployment.yaml"
test -f "${base}/backend-service.yaml"
test -f "${base}/frontend-service.yaml"
test -f "${base}/backend-ingress.yaml"
test -f "${base}/frontend-ingress.yaml"
test -f "${base}/migration-job.yaml"
test -f "${base}/backend-hpa.yaml"
test -f "deployments/k8s/kind/kind-config.yaml"
test -f "scripts/kind_deploy.sh"
test -f "deployments/k8s/overlays/dev/kustomization.yaml"
test -f "deployments/k8s/overlays/staging/kustomization.yaml"
test -f "deployments/k8s/overlays/prod/kustomization.yaml"

kubectl kustomize "${base}" > "${rendered}"

grep -q "kind: Deployment" "${rendered}"
grep -q "name: contentflow-backend" "${rendered}"
grep -q "name: contentflow-frontend" "${rendered}"
grep -q "name: contentflow-worker" "${rendered}"
grep -q "name: contentflow-scheduler" "${rendered}"
grep -q "kind: Service" "${rendered}"
grep -q "kind: Ingress" "${rendered}"
grep -q "contentflow-frontend-tls" "${rendered}"
grep -q "kind: Job" "${rendered}"
grep -q "kind: HorizontalPodAutoscaler" "${rendered}"
grep -q "CONTENTFLOW_APP_MODE" "${rendered}"
grep -q "/healthz" "${rendered}"
grep -q "/readyz" "${rendered}"

kubectl kustomize deployments/k8s/overlays/dev >/tmp/contentflow-k8s-dev.yaml
kubectl kustomize deployments/k8s/overlays/staging >/tmp/contentflow-k8s-staging.yaml
kubectl kustomize deployments/k8s/overlays/prod >/tmp/contentflow-k8s-prod.yaml
grep -q "kind: ExternalSecret" /tmp/contentflow-k8s-prod.yaml
! grep -q "^kind: Secret$" /tmp/contentflow-k8s-prod.yaml
