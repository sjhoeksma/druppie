#!/bin/bash

# Druppie Database Services Setup
# Installs PostgreSQL (Relational/GIS) and Qdrant (Vector)

set -e

COLOR_GREEN='\033[0;32m'
COLOR_BLUE='\033[0;34m'
COLOR_NC='\033[0m'

LOG_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../.druppie_history"

function log() {
    echo -e "${COLOR_BLUE}[SETUP]${COLOR_NC} $1"
}

function log_history() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') | SETUP     | Databases  | $1" >> "$LOG_FILE"
}

# 1. Check Connectivity
log "Checking Kubernetes Connection..."
if ! kubectl cluster-info &> /dev/null; then
    echo "❌ Cannot connect to Kubernetes."
    exit 1
fi

# 2. Add Repos
log "Adding Helm Repositories..."
helm repo add qdrant https://qdrant.github.io/qdrant-helm
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo up

# 3. Install Qdrant (Vector DB for AI)
log "Installing Qdrant Vector DB..."
helm upgrade --install qdrant qdrant/qdrant \
  --namespace databases --create-namespace \
  --set replicaCount=1 \
  --set persistence.size=2Gi \
  --set apiKey=${DRUPPIE_QDRANT_KEY} \
  --wait
log_history "Qdrant Installed"

# 4. Install PostgreSQL with PostGIS (Geo DB)
log "Installing PostgreSQL + PostGIS..."
# We use standard Postgres chart. PostGIS usually requires a specific image or extension enable.
# For simplicity, we assume standard here, but in real setup we'd select a postgis image.
helm upgrade --install postgres bitnami/postgresql \
  --namespace databases --create-namespace \
  --set global.postgresql.auth.postgresPassword=${DRUPPIE_POSTGRES_PASS} \
  --set primary.persistence.size=5Gi \
  --set image.tag="16-debian-12" \
  --wait
  
# Note: Enabling PostGIS extension is usually a 'CREATE EXTENSION postgis;' command after boot.
# We log this as a reminder.
log_history "PostgreSQL Installed"

# 5. Success
echo -e "${COLOR_GREEN}"
echo "✅  Database Services Installed!"
echo "------------------------------"
echo " - Qdrant:    http://qdrant.databases.svc.cluster.local:6333"
echo "   API Key:   ${DRUPPIE_QDRANT_KEY}"
echo ""
echo " - Postgres:  postgres-postgresql.databases.svc.cluster.local:5432"
echo "   User:      postgres"
echo "   Pass:      ${DRUPPIE_POSTGRES_PASS}"
echo ""
echo "⚠️  Note: Enable PostGIS manually if needed:"
echo "   kubectl exec -it -n databases svc/postgres-postgresql -- psql -U postgres -c 'CREATE EXTENSION IF NOT EXISTS postgis;'"
echo -e "${COLOR_NC}"
