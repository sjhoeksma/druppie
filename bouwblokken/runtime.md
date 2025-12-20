# Runtime Interface

## 🎯 Doelstelling
Dit bouwblok definieert de **interface** naar de Runtime omgeving. In de Azure Foundry architectuur bestaat deze runtime uit twee delen: de **Agent Runtime** (Azure AI Agent Service) voor de intelligentie, en de **Container Runtime** (Kubernetes/ACA) voor de workloads.

## 📋 Functionele Specificaties

### 1. Agent Runtime (Azure AI Agent Service)
- **Host**: De beheerde omgeving waar AI-agenten (Threads, Runs) worden uitgevoerd.
- **State Management**: Automatisch beheer van conversatie-state en context (persistentie).
- **Tool Execution**: Veilige uitvoering van code (Code Interpreter) en API-calls via OpenAPI.

### 2. Deployment Management (Container Runtime)
- **Manifest Application**: Vertalen van abstracte deployment intenties naar concrete Kubernetes manifests of Azure Container Apps.
- **Rollout Control**: Beheren van updates en rollbacks (Blue/Green).

### 3. Resource Provisioning aka "Dynamic Slots"
- **Namespace on Demand**: Aanmaken van geïsoleerde omgevingen voor nieuwe projecten.
- **Quota Management**: Toewijzen van CPU/Memory limieten.

## 2. Security & Compliance
- **Entra Agent ID**: Elke agent draait onder zijn eigen Managed Identity.
- **Network Isolation**: Alle communicatie verloopt via Private Endpoints binnen het VNet.

## 3. Geselecteerde Kubernetes Stack (RKE2)
Voor de **Container Runtime** is gekozen voor **RKE2** (Rancher Kubernetes Engine 2), ook wel bekend als RKE Government.

### 💡 Waarom RKE2?
- **Security-First**: RKE2 is standaard gehard volgens de CIS Kubernetes Benchmark en is FIPS 140-2 compliant. Dit is essentieel voor overheidsinstellingen zoals Waterschappen.
- **Lichtgewicht & Robuust**: Het bundelt alle componenten in één binary maar gebruikt de zuivere, moderne CRI (Container Runtime Interface) standaarden (containerd).
- **Eenvoudig Beheer**: Het automatiseert certificaatbeheer en etcd snapshots.

### ⚙️ Configuratie (`config.yaml`)
Een typische, veilige configuratie voor een RKE2 node binnen Druppie:

```yaml
# RKE2 Server Config
write-kubeconfig-mode: "0644"
tls-san:
  - "k8s-api.druppie.nl"
cni: "canal" # Calico + Flannel voor Network Policy enforcement
profile: "cis-1.23" # Activeer CIS Hardening Profile
selinux: true
token: "SECRET_CLUSTER_TOKEN"
system-default-registry: "registry.druppie.nl" # Gebruik de interne, scanned registry
kube-apiserver-arg:
  - "audit-log-path=/var/lib/rancher/rke2/server/audit.log"
  - "audit-policy-file=/etc/rancher/rke2/audit-policy.yaml" # Audit Logging voor Compliance
```

## 🏗️ Relaties
- **Verwijst naar**: [Runtime Domein](../runtime/overview.md).
- **Wordt gebruikt door**: [Druppie Core](./druppie_core.md) en [Build Plane](./build_plane.md).
