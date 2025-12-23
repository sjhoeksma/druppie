#!/bin/bash

# Druppie IAM Setup
# Installs Keycloak (Identity Provider)

set -e

COLOR_GREEN='\033[0;32m'
COLOR_BLUE='\033[0;34m'
COLOR_NC='\033[0m'

LOG_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../.druppie_history"

function log() {
    echo -e "${COLOR_BLUE}[SETUP]${COLOR_NC} $1"
}

function log_history() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') | SETUP     | IAM        | $1" >> "$LOG_FILE"
}

# 1. Check Connectivity
log "Checking Kubernetes Connection..."
if ! kubectl cluster-info &> /dev/null; then
    echo "❌ Cannot connect to Kubernetes."
    exit 1
fi

# 2. Add Repos
log "Adding Helm Repositories..."
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo up

# 3. Install Keycloak
log "Installing Keycloak..."
helm upgrade --install keycloak bitnami/keycloak \
  --namespace iam --create-namespace \
  --set auth.adminUser=admin \
  --set auth.adminPassword=${DRUPPIE_KEYCLOAK_PASS} \
  --set production=false \
  --set proxy=edge \
  --set service.type=ClusterIP \
  --set postgresql.enabled=false \
  --set externalDatabase.host=postgres-postgresql.databases.svc.cluster.local \
  --set externalDatabase.port=5432 \
  --set externalDatabase.user=postgres \
  --set externalDatabase.database=postgres \
  --set externalDatabase.password=${DRUPPIE_POSTGRES_PASS} \
  --set image.repository=bitnamilegacy/keycloak \
  --set image.tag=26.3.3-debian-12-r0 \
  --wait
log_history "Keycloak Installed"

# 5. Success
echo -e "${COLOR_GREEN}"
echo "✅  IAM Services Installed!"
echo "-------------------------"
echo " - Keycloak:  http://keycloak.iam.svc.cluster.local:80"
echo "   User:      admin"
echo "   Pass:      ${DRUPPIE_KEYCLOAK_PASS}"
echo ""
echo "⚠️  Note: Port-forward to access locally:"
echo "   kubectl port-forward svc/keycloak -n iam 8080:80"
echo -e "${COLOR_NC}"
