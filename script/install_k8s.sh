#!/bin/bash

# Druppie Kubernetes Installer (RKE2)
# Supports: Server (Control Plane) and Workstation (Single Node Dev)

set -e

COLOR_GREEN='\033[0;32m'
COLOR_BLUE='\033[0;34m'
COLOR_RED='\033[0;31m'
COLOR_NC='\033[0m'

# Log File Location (Project Root)
LOG_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../.druppie_history"

function log() {
    echo -e "${COLOR_BLUE}[DRUPPIE]${COLOR_NC} $1"
}

function log_history() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') | INSTALL   | Kubernetes | $1" >> "$LOG_FILE"
}

function error() {
    echo -e "${COLOR_RED}[ERROR]${COLOR_NC} $1"
    exit 1
}

# Check Root (Only required for RKE2/Linux)
IS_ROOT=false
if [[ $EUID -eq 0 ]]; then
   IS_ROOT=true
fi

OS="$(uname)"

# Selection Menu
echo -e "${COLOR_GREEN}Select Installation Profile:${COLOR_NC}"
echo "1) Local Dev (k3d - Docker Required - macOS/Linux)"
echo "2) Install Rancher UI (Requires existing Cluster - Helm)"
echo "3) Workstation (RKE2 - Single Node - Linux Only)"
echo "4) Server (RKE2 - Control Plane - Linux Only)"
read -p "Choice [1-4]: " PROFILE_OPT

# Validation
if [[ "$PROFILE_OPT" == "3" || "$PROFILE_OPT" == "4" ]]; then
    if [[ "$OS" == "Darwin" ]]; then
        error "RKE2 (Options 3/4) is only supported on Linux. Use Option 1 (k3d) for macOS."
    fi
    if [ "$IS_ROOT" = false ]; then
        error "RKE2 installation requires root (sudo)."
    fi
fi

if [ "$PROFILE_OPT" == "1" ]; then
    log "Checking Docker..."
    if ! command -v docker &> /dev/null; then
        error "Docker is not installed or not in PATH."
    fi
    if ! docker info &> /dev/null; then
        error "Docker daemon is not running."
    fi

    log "Checking/Installing k3d..."
    if ! command -v k3d &> /dev/null; then
        if [[ "$OS" == "Darwin" ]]; then
            log "Installing k3d via Homebrew..."
            brew install k3d
        else
            log "Installing k3d via Script..."
            curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash
        fi
    else
        log "k3d is already installed."
    fi

    log "Checking/Installing kubectl..."
    if ! command -v kubectl &> /dev/null; then
        if [[ "$OS" == "Darwin" ]]; then
             log "Installing kubectl via Homebrew..."
             brew install kubectl
        else
             log "Installing kubectl..."
             curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
             chmod +x kubectl
             sudo mv kubectl /usr/local/bin/
        fi
    else
        log "kubectl is already installed."
    fi

    log "Checking/Installing Helm..."
    if ! command -v helm &> /dev/null; then
        if [[ "$OS" == "Darwin" ]]; then
            log "Installing Helm via Homebrew..."
            brew install helm
        else
            log "Installing Helm..."
            curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
        fi
    fi

    log "Creating/Updating Cluster 'druppie-dev'..."
    # Create cluster with Ingress ports mapped
    # - 80:80 (HTTP Ingress)
    # - 443:443 (HTTPS Ingress)
    k3d cluster create druppie-dev \
        --api-port 6443 \
        -p "80:80@loadbalancer" \
        -p "443:443@loadbalancer" \
        --k3s-arg "--disable=traefik@server:0" \
        --wait

    log "Cluster Ready! 🚀"
    log "Run: kubectl get nodes"
    log_history "k3d (druppie-dev)"
    read -p "Druk op Enter..."
    exit 0
fi

if [ "$PROFILE_OPT" == "2" ]; then
    log "Checking Helm..."
    if ! command -v helm &> /dev/null; then
         error "Helm is likely required but not found. Please install Helm or run Option 1 first."
    fi

    # Check for Token
    if [ -z "$DRUPPIE_RANCHER_TOKEN" ]; then
        read -p "Enter Rancher Bootstrap Password (e.g. admin): " DRUPPIE_RANCHER_TOKEN
    fi

    log "Installing Cert-Manager (Required for Rancher)..."
    helm repo add jetstack https://charts.jetstack.io
    helm repo update
    helm upgrade --install cert-manager jetstack/cert-manager \
      --namespace cert-manager --create-namespace \
      --set crds.enabled=true \
      --wait

    log "Installing Rancher UI..."
    helm repo add rancher-stable https://releases.rancher.com/server-charts/stable
    helm repo update

    # Install Rancher
    log "Deploying Rancher..."
 helm upgrade --install rancher rancher-stable/rancher \
      --namespace cattle-system --create-namespace \
      --set hostname=localhost \
      --set bootstrapPassword=${DRUPPIE_RANCHER_TOKEN} \
      --set ingress.tls.source=rancher \
      --set replicas=1 \
      --set ingress.ingressClassName=kong \
      --wait

    log "Waiting for Rancher to be ready..."
    kubectl -n cattle-system rollout status deploy/rancher

    log "Rancher UI Installed! 🤠"
    log "Access URL: https://localhost"
    log "Login: ${DRUPPIE_RANCHER_TOKEN}"
    log "(Note: Accept the self-signed certificate warning)"
    
    log_history "Rancher UI Installed"
    read -p "Druk op Enter..."
    exit 0
fi

# RKE2 Logic (Linux Only)
RKE2_CONFIG_DIR="/etc/rancher/rke2"
CONFIG_FILE="$RKE2_CONFIG_DIR/config.yaml"

mkdir -p $RKE2_CONFIG_DIR

if [ "$PROFILE_OPT" == "3" ]; then
    log "Configuring for WORKSTATION..."
    # Workstation: Minimal resource usage
    cat <<EOF > $CONFIG_FILE
write-kubeconfig-mode: "0644"
tls-san:
  - "druppie.local"
  - "127.0.0.1"
disable:
  - rke2-ingress-nginx
profile: "cis-1.23"
EOF

elif [ "$PROFILE_OPT" == "4" ]; then
    log "Configuring for SERVER (Production)..."
    if [ -z "$CLUSTER_TOKEN" ]; then
        if [ ! -z "$DRUPPIE_K8S_TOKEN" ]; then
            CLUSTER_TOKEN="$DRUPPIE_K8S_TOKEN"
            log "Using Token from Environment."
        else
            read -p "Enter Cluster Token (secret): " CLUSTER_TOKEN
        fi
    fi
    read -p "Enter Generic S3 Endpoint (for etcd snapshots) [optional]: " S3_ENDPOINT
    
    cat <<EOF > $CONFIG_FILE
write-kubeconfig-mode: "0644"
token: "$CLUSTER_TOKEN"
cni: "canal"
profile: "cis-1.23"
selinux: true
kube-apiserver-arg:
  - "audit-log-path=/var/lib/rancher/rke2/server/audit.log"
  - "audit-policy-file=/etc/rancher/rke2/audit-policy.yaml"
EOF
    
    cat <<EOF > $RKE2_CONFIG_DIR/audit-policy.yaml
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
  - level: Metadata
EOF

    if [ ! -z "$S3_ENDPOINT" ]; then
        echo "etcd-s3: true" >> $CONFIG_FILE
        echo "etcd-s3-endpoint: $S3_ENDPOINT" >> $CONFIG_FILE
    fi
fi

# Install RKE2
log "Downloading and Installing RKE2..."
curl -sfL https://get.rke2.io | sh -

# Enable & Start
log "Starting RKE2 Server..."
systemctl enable rke2-server.service
systemctl start rke2-server.service

# Symlink kubectl for convenience
if [ ! -f /usr/local/bin/kubectl ]; then
    log "Symlinking RKE2 kubectl to /usr/local/bin/kubectl..."
    ln -s /var/lib/rancher/rke2/bin/kubectl /usr/local/bin/kubectl
fi

# Validating
log "Waiting for Node to be Ready..."
export KUBECONFIG=/etc/rancher/rke2/rke2.yaml
for i in {1..30}; do
    if /var/lib/rancher/rke2/bin/kubectl get nodes &> /dev/null; then
        break
    fi
    sleep 2
done

/var/lib/rancher/rke2/bin/kubectl get nodes

log "Installation Complete! 🚀"
log "Kubeconfig location: /etc/rancher/rke2/rke2.yaml"
if [ "$PROFILE_OPT" == "3" ]; then
    log "You can copy this to ~/.kube/config on your host."
    log_history "RKE2 (Workstation)"
elif [ "$PROFILE_OPT" == "4" ]; then
    log_history "RKE2 (Server)"
fi
