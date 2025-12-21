#!/bin/bash

# Druppie Security Services Setup
# Installs Trivy (Scanning) and SonarQube (Static Analysis)

set -e

COLOR_GREEN='\033[0;32m'
COLOR_BLUE='\033[0;34m'
COLOR_NC='\033[0m'

LOG_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../.druppie_history"

function log() {
    echo -e "${COLOR_BLUE}[SETUP]${COLOR_NC} $1"
}

function log_history() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') | SETUP     | Security   | $1" >> "$LOG_FILE"
}

# 1. Check Connectivity
log "Checking Kubernetes Connection..."
if ! kubectl cluster-info &> /dev/null; then
    echo "❌ Cannot connect to Kubernetes."
    exit 1
fi

# 2. Add Repos
log "Adding Helm Repositories..."
helm repo add aqua https://aquasecurity.github.io/helm-charts/
helm repo add sonarqube https://SonarSource.github.io/helm-chart-sonarqube
helm repo up

# 3. Install Trivy Operator (Continuous Scanning)
log "Installing Trivy Operator..."
helm upgrade --install trivy-operator aqua/trivy-operator \
  --namespace security-system --create-namespace \
  --set compliance.cron="0 */6 * * *" \
  --wait
log_history "Trivy Operator Installed"

# 4. Install SonarQube (Code Quality)
log "Installing SonarQube..."
helm upgrade --install sonarqube sonarqube/sonarqube \
  --namespace security-system --create-namespace \
  --set edition=community \
  --set persistence.enabled=true \
  --set persistence.size=5Gi \
  --set service.type=ClusterIP \
  --wait
log_history "SonarQube Installed"

# 5. Success
echo -e "${COLOR_GREEN}"
echo "✅  Security Services Installed!"
echo "------------------------------"
echo " - Trivy:     Background Operator (Check 'trivy-operator' pod)"
echo " - SonarQube: http://sonarqube-sonarqube.security-system.svc.cluster.local:9000"
echo "   User:      admin"
echo "   Pass:      admin (Requires change on first login)"
echo ""
echo "⚠️  Note: Port-forward to access locally:"
echo "   kubectl port-forward svc/sonarqube-sonarqube -n security-system 9000:9000"
echo -e "${COLOR_NC}"
