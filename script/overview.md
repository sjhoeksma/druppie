# Scripts & Automatisering

Hier bewaren we herbruikbare scripts en quick-start commando's die gebruikt kunnen worden voor beheer, setup, of demo doeleinden.

## 🚀 Hoofdmenu
Het makkelijkste startpunt is de Master CLI in de root:
*   **[./druppie.sh](../druppie.sh)**: Interactief menu voor installatie, bootstrap en beheer.

## 📂 Scripts per Categorie

### 🏗️ Infrastructuur
*   **[install_k8s.sh](./install_k8s.sh)**: Installeert Kubernetes (RKE2 voor Linux/Prod, k3d voor Mac/Dev).
*   **[uninstall_k8s.sh](./uninstall_k8s.sh)**: Verwijdert de Kubernetes cluster.

### 📦 Platform Services
*   **[setup_dev_env.sh](./setup_dev_env.sh)**: **Base Layer**. Installeert Flux (GitOps), Kyverno (Policy), Tekton (CI) en Kong (Gateway).
*   **[setup_iam.sh](./setup_iam.sh)**: **IAM**. Installeert Keycloak (Identity Provider).
*   **[setup_observability.sh](./setup_observability.sh)**: **Observability**. Installeert de LGTM stack (Loki, Grafana, Tempo, Prometheus).

### 🛠️ Applicatie Services
*   **[setup_data_tools.sh](./setup_data_tools.sh)**: **Data**. Installeert Gitea (Git) en MinIO (S3 Lake).
*   **[setup_databases.sh](./setup_databases.sh)**: **Storage**. Installeert PostgreSQL (SQL/GIS) en Qdrant (Vector).
*   **[setup_security_tools.sh](./setup_security_tools.sh)**: **Security**. Installeert Trivy (Scanning) en SonarQube (Quality).
*   **[setup_gis.sh](./setup_gis.sh)**: **GIS**. Installeert GeoServer (Maps), GeoNode (Portal) en NodeODM (Drone).

### 🧪 Demo & Simulatie
*   `simulate_user_login.py` (Concept)
*   `trigger_drone_flow.sh` (Concept)

## 🤖 Automatisering
Deze scripts zijn idempotent en kunnen worden aangeroepen door:
1.  **Engineers**: Via `druppie.sh` of direct.
2.  **Builder Agent**: Om omgevingen te provisionen.
3.  **Argo Workflows**: Als onderdeel van een grotere keten.
