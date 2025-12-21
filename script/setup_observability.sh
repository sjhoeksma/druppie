#!/bin/bash

# Druppie Observability Setup
# Installs the LGTM Stack (Loki, Grafana, Tempo, Mimir/Prometheus)

set -e

COLOR_GREEN='\033[0;32m'
COLOR_BLUE='\033[0;34m'
COLOR_NC='\033[0m'

LOG_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../.druppie_history"

function log() {
    echo -e "${COLOR_BLUE}[SETUP]${COLOR_NC} $1"
}

function log_history() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') | SETUP     | Observability | $1" >> "$LOG_FILE"
}

# 1. Check Connectivity
log "Checking Kubernetes Connection..."
if ! kubectl cluster-info &> /dev/null; then
    echo "❌ Cannot connect to Kubernetes."
    exit 1
fi

# 2. Add Repos
log "Adding Helm Repositories..."
helm repo add grafana https://grafana.github.io/helm-charts
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo up

# 3. Install Prometheus (Metrics)
log "Installing Prometheus..."
helm upgrade --install prometheus prometheus-community/prometheus \
  --namespace observability --create-namespace \
  --set alertmanager.enabled=false \
  --set server.persistentVolume.size=5Gi \
  --wait
log_history "Prometheus Installed"

# 4. Install Loki (Logs)
log "Installing Loki (Monolithic mode for Dev)..."
helm upgrade --install loki grafana/loki-stack \
  --namespace observability --create-namespace \
  --set fluent-bit.enabled=true \
  --set promtail.enabled=true \
  --wait
log_history "Loki & Promtail Installed"

# 5. Install Grafana (Dashboarding)
log "Installing Grafana..."
helm upgrade --install grafana grafana/grafana \
  --namespace observability --create-namespace \
  --set adminPassword=${DRUPPIE_GRAFANA_PASS} \
  --set service.type=LoadBalancer \
  --set persistence.enabled=true \
  --set persistence.size=2Gi \
  --wait
log_history "Grafana Installed"

# 6. Install Tempo (Tracing - optional/lightweight)
log "Installing Tempo..."
helm upgrade --install tempo grafana/tempo \
  --namespace observability --create-namespace \
  --set persistence.enabled=false \
  --wait
log_history "Tempo Installed"

# 7. Success
echo -e "${COLOR_GREEN}"
echo "✅  Observability Stack (LGTM) Installed!"
echo "---------------------------------------"
echo " - Grafana:   http://grafana.observability.svc.cluster.local:80"
echo "   User:      admin"
echo "   Pass:      ${DRUPPIE_GRAFANA_PASS}"
echo "   Data:      Prometheus (Metrics), Loki (Logs), Tempo (Traces)"
echo ""
echo "⚠️  Note: Port-forward to access locally:"
echo "   kubectl port-forward svc/grafana -n observability 3000:80"
echo -e "${COLOR_NC}"
