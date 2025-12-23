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
  --set compliance.cron="0 0 * * *" \
  --set operator.scanJobConcurrencyLimit=1 \
  --set operator.scanJobTimeout=5m \
  --set operator.vulnerabilityScannerScanOnlyCurrentRevisions=true \
  --set operator.replicas=1 \
  --set targetNamespaces="default" \
  --set excludeNamespaces="kube-system\,security-system\,cattle-system\,flux-system\,cert-manager\,local-path-storage" \
  --set operator.backgroundScanning.enabled=false \
  --set vulnerabilityReport.scanAll=false \
  --set trivy.ignoreUnfixed=true \
  --set trivy.resources.requests.cpu=50m \
  --set trivy.resources.requests.memory=128Mi \
  --set trivy.resources.limits.memory=256Mi \
  --wait
log_history "Trivy Operator Installed (Restricted Scope)"



# 4. Install SonarQube (Code Quality) - Manual Manifest
log "Installing SonarQube (via Manifest)..."

log "Creating SonarQube Database..."
# Create dedicated database to avoid version conflicts and clean slate
kubectl exec -n databases svc/postgres-postgresql -- env PGPASSWORD=${DRUPPIE_POSTGRES_PASS} psql -U postgres -c "CREATE DATABASE sonarqube;" || true

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: sonarqube-data
  namespace: security-system
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 5Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sonarqube
  namespace: security-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: sonarqube
  template:
    metadata:
      labels:
        app: sonarqube
    spec:
      securityContext:
        fsGroup: 1000
      initContainers:
      - name: init-sysctl
        image: busybox
        command: ["sysctl", "-w", "vm.max_map_count=262144"]
        securityContext:
          privileged: true
      containers:
      - name: sonarqube
        image: sonarqube:community
        ports:
        - containerPort: 9000
        env:
        - name: SONAR_JDBC_URL
          value: "jdbc:postgresql://postgres-postgresql.databases.svc.cluster.local:5432/sonarqube"
        - name: SONAR_JDBC_USERNAME
          value: "postgres"
        - name: SONAR_JDBC_PASSWORD
          value: "${DRUPPIE_POSTGRES_PASS}"
        resources:
          requests:
            memory: "1Gi"
            cpu: "500m"
          limits:
            memory: "2Gi"
        volumeMounts:
        - mountPath: /opt/sonarqube/data
          name: sonarqube-data
      volumes:
      - name: sonarqube-data
        persistentVolumeClaim:
          claimName: sonarqube-data
---
apiVersion: v1
kind: Service
metadata:
  name: sonarqube-sonarqube
  namespace: security-system
spec:
  selector:
    app: sonarqube
  ports:
  - port: 9000
    targetPort: 9000
    name: http
  type: ClusterIP
EOF

log "Waiting for SonarQube to start..."
kubectl rollout status deployment/sonarqube -n security-system --timeout=300s
log_history "SonarQube Installed (Manual Manifest)"

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
