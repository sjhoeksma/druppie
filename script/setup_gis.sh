#!/bin/bash

# Druppie GIS Services Setup
# Installs GeoServer (Map Server) and WebODM (Drone Mapping)

set -e

COLOR_GREEN='\033[0;32m'
COLOR_BLUE='\033[0;34m'
COLOR_NC='\033[0m'

LOG_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../.druppie_history"

function log() {
    echo -e "${COLOR_BLUE}[SETUP]${COLOR_NC} $1"
}

function log_history() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') | SETUP     | GIS        | $1" >> "$LOG_FILE"
}

# 1. Check Connectivity
log "Checking Kubernetes Connection..."
if ! kubectl cluster-info &> /dev/null; then
    echo "❌ Cannot connect to Kubernetes."
    exit 1
fi

# 2. Add Repos
log "Adding Helm Repositories..."
helm repo add geoserver https://geosolutions-it.github.io/helm-charts/
# GeoNode often doesn't have a stable single Helm chart, we use a simple k8s manifest approach for this simplified setup.
helm repo up

# 3. Create Namespace
kubectl create namespace gis --dry-run=client -o yaml | kubectl apply -f -

# 4. Install GeoServer
log "Installing GeoServer..."
helm upgrade --install geoserver geoserver/geoserver \
  --namespace gis \
  --set service.type=LoadBalancer \
  --wait
log_history "GeoServer Installed"

# 5. Install GeoNode (GIS Portal)
# GeoNode requires Django, Celery, Postgres, and GeoServer integration.
# This simplifies it to the Core Django app linked to the Postgres we installed earlier (conceptually).
log "Installing GeoNode (Portal)..."
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: geonode
  namespace: gis
spec:
  replicas: 1
  selector:
    matchLabels:
      app: geonode
  template:
    metadata:
      labels:
        app: geonode
    spec:
      containers:
      - name: geonode
        image: geonode/geonode:4.1.3
        env:
        - name: DATABASE_URL
          value: "postgres://postgres:${DRUPPIE_POSTGRES_PASS}@postgres-postgresql.databases.svc.cluster.local:5432/geonode"
        - name: GEOSERVER_PUBLIC_LOCATION
          value: "http://geoserver.gis.svc.cluster.local:8080/geoserver/"
        ports:
        - containerPort: 8000
---
apiVersion: v1
kind: Service
metadata:
  name: geonode
  namespace: gis
spec:
  selector:
    app: geonode
  ports:
  - port: 80
    targetPort: 8000
  type: LoadBalancer
EOF
log_history "GeoNode Installed"

# 5. Install WebODM (Lightning/NodeODM)
# WebODM is complex on K8s (Storage/Postgres/Redis). 
# For this 'Light' script, we deploy a simplified NodeODM (processing node) and references.
# In a real PROD setup this would be full WebODM stack.
log "Installing NodeODM (Drone Processing Engine)..."
# Using raw manifest for simplicity as no official stable Helm chart exists for simple setups
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nodeodm
  namespace: gis
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nodeodm
  template:
    metadata:
      labels:
        app: nodeodm
    spec:
      containers:
      - name: nodeodm
        image: opendronemap/nodeodm:latest
        ports:
        - containerPort: 3000
---
apiVersion: v1
kind: Service
metadata:
  name: nodeodm
  namespace: gis
spec:
  selector:
    app: nodeodm
  ports:
  - port: 3000
    targetPort: 3000
  type: ClusterIP
EOF
log_history "NodeODM Installed"

# 6. Success
echo -e "${COLOR_GREEN}"
echo "✅  GIS Services Installed!"
echo "-------------------------"
echo " - GeoServer: http://geoserver.gis.svc.cluster.local:8080/geoserver"
echo "   User:      admin"
echo "   Pass:      geoserver"
echo ""
echo " - GeoNode:   http://geonode.gis.svc.cluster.local:80"
echo "   (Map Portal - Requires DB)"
echo ""
echo " - NodeODM:   http://nodeodm.gis.svc.cluster.local:3000"
echo "   (Drone Processing API)"
echo ""
echo "⚠️  Note: Port-forward to access locally:"
echo "   kubectl port-forward svc/geoserver -n gis 8080:8080"
echo "   kubectl port-forward svc/geonode -n gis 8000:80"
echo "   kubectl port-forward svc/nodeodm -n gis 3000:3000"
echo -e "${COLOR_NC}"
