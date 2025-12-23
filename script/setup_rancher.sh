#!/bin/bash

# Druppie Rancher UI Setup
# Installs Cert-Manager and Rancher UI on an existing Kubernetes cluster

set -e

COLOR_GREEN='\033[0;32m'
COLOR_BLUE='\033[0;34m'
COLOR_NC='\033[0m'

LOG_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../.druppie_history"

function log() {
    echo -e "${COLOR_BLUE}[SETUP]${COLOR_NC} $1"
}

function log_history() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') | SETUP     | Rancher    | $1" >> "$LOG_FILE"
}

# 1. Check Connectivity
log "Checking Kubernetes Connection..."
if ! kubectl cluster-info &> /dev/null; then
    echo "❌ Cannot connect to Kubernetes."
    exit 1
fi

# Check for Token
if [ -z "$DRUPPIE_RANCHER_TOKEN" ]; then
    echo "⚠️  DRUPPIE_RANCHER_TOKEN is not set. Defaulting to 'admin' for Dev."
    DRUPPIE_RANCHER_TOKEN="admin"
fi

# 2. Add Repos
log "Adding Helm Repositories..."
helm repo add jetstack https://charts.jetstack.io
helm repo add rancher-stable https://releases.rancher.com/server-charts/stable
helm repo up

# 3. Install Cert-Manager (Required for Rancher)
log "Installing Cert-Manager (Required for Rancher)..."
helm upgrade --install cert-manager jetstack/cert-manager \
    --namespace cert-manager --create-namespace \
    --set crds.enabled=true \
    --wait
log_history "Cert-Manager Installed"

# 4. Install Rancher
log "Installing Rancher UI..."
log "Deploying Rancher..."

helm upgrade --install rancher rancher-stable/rancher \
    --namespace cattle-system --create-namespace \
    --set hostname=localhost \
    --set bootstrapPassword=${DRUPPIE_RANCHER_TOKEN} \
    --set ingress.tls.source=rancher \
    --set replicas=1 \
    --set ingress.ingressClassName=kong \
    --set ingress.enabled=false \
    --set "extraEnv[0].name=CATTLE_SERVER_URL" \
    --set "extraEnv[0].value=https://rancher.${DRUPPIE_DOMAIN}" \
    --wait

log "Configuring Rancher Ingress at https://rancher.${DRUPPIE_DOMAIN}..."
cat <<EOF | kubectl apply -f -
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: rancher-custom-ingress
  namespace: cattle-system
spec:
  ingressClassName: kong
  rules:
  - host: rancher.${DRUPPIE_DOMAIN}
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: rancher
            port:
              number: 80
EOF

log "Waiting for Rancher to be ready..."
kubectl -n cattle-system rollout status deploy/rancher

log "Rancher UI Installed! 🤠"
log "Access URL: https://rancher.${DRUPPIE_DOMAIN}"
log "Login: ${DRUPPIE_RANCHER_TOKEN}"
log "(Note: Accept the self-signed certificate warning. Ensure 'rancher.${DRUPPIE_DOMAIN}' resolves. Localhost assumed if DRUPPIE_DOMAIN=localhost)"

log_history "Rancher UI Installed"

# 5. Success
echo -e "${COLOR_GREEN}"
echo "✅  Rancher UI Installed Successfully!"
echo "--------------------------------------"
echo "URL: https://rancher.${DRUPPIE_DOMAIN}"
echo "Login: admin / ${DRUPPIE_RANCHER_TOKEN}"
echo -e "${COLOR_NC}"
