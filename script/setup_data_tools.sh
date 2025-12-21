#!/bin/bash

# Druppie Data & Versioning Setup
# Installs Gitea (Version Control) and MinIO (Data Lake)

set -e

COLOR_GREEN='\033[0;32m'
COLOR_BLUE='\033[0;34m'
COLOR_NC='\033[0m'

LOG_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../.druppie_history"

function log() {
    echo -e "${COLOR_BLUE}[SETUP]${COLOR_NC} $1"
}

function log_history() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') | SETUP     | DataTools  | $1" >> "$LOG_FILE"
}

# 1. Check Connectivity
log "Checking Kubernetes Connection..."
if ! kubectl cluster-info &> /dev/null; then
    echo "❌ Cannot connect to Kubernetes."
    exit 1
fi

# 2. Add Repos
log "Adding Helm Repositories..."
helm repo add gitea-charts https://dl.gitea.io/charts/
helm repo add minio https://charts.min.io/
helm repo up

# 3. Install Gitea (Lightweight Git Server)
log "Installing Gitea..."
helm upgrade --install gitea gitea-charts/gitea \
  --namespace gitea --create-namespace \
  --set gitea.admin.username=druppie_admin \
  --set gitea.admin.password=${DRUPPIE_GITEA_PASS} \
  --set persistence.size=1Gi \
  --wait
log_history "Gitea Installed"

# 4. Install MinIO (S3 Compatible Storage)
log "Installing MinIO..."
helm upgrade --install minio minio/minio \
  --namespace minio --create-namespace \
  --set rootUser=admin \
  --set rootPassword=${DRUPPIE_MINIO_PASS} \
  --set persistence.size=5Gi \
  --wait
log_history "MinIO Installed"

# 5. Success
echo -e "${COLOR_GREEN}"
echo "✅  Data Services Installed!"
echo "--------------------------"
echo " - Gitea:     http://gitea-http.gitea.svc.cluster.local:3000"
echo "   User:      druppie_admin"
echo "   Pass:      ${DRUPPIE_GITEA_PASS}"
echo ""
echo " - MinIO:     http://minio.minio.svc.cluster.local:9000"
echo "   User:      admin"
echo "   Pass:      ${DRUPPIE_MINIO_PASS}"
echo ""
echo "⚠️  Note: Port-forward to access locally:"
echo "   kubectl port-forward svc/gitea-http -n gitea 3000:3000"
echo "   kubectl port-forward svc/minio -n minio 9000:9000"
echo -e "${COLOR_NC}"
