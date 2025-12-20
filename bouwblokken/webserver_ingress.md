# Webserver & Certificaten (Ingress)

## 🎯 Selectie: NGINX Ingress & cert-manager
Voor het ontsluiten van webapplicaties en API's naar de buitenwereld, inclusief geautomatiseerd HTTPS-certificaatbeheer, kiezen we voor de gouden standaard binnen het Kubernetes ecosysteem: **NGINX Ingress Controller** in combinatie met **cert-manager**.

### 💡 Onderbouwing van de Keuze
Deze combinatie is essentieel voor een veilig en toegankelijk platform:

1.  **Centraal Toegangspunt**: In plaats van elke applicatie zijn eigen webserver en loadbalancer te geven (wat duur en complex is), regelt de Ingress Controller alle inkomende verkeer centraal. Dit is efficiënt en veilig.
2.  **Automated Security (HTTPS)**: `cert-manager` neemt het volledige beheer van TLS-certificaten uit handen. Het vraagt certificaten aan (bijv. via Let's Encrypt of een interne CA), valideert ze, en ververst ze automatisch voordat ze verlopen. Dit elimineert menselijke fouten en outages door verlopen certificaten.
3.  **Kubernetes Native**: Configuratie gebeurt via standaard Kubernetes `Ingress` resources. Dit past perfect in de *Spec-Driven* werkwijze van Druppie: de infrastructuur is code.
4.  **Flexibele Routing**: NGINX biedt geavanceerde mogelijkheden zoals path-based routing (bijv. `/api` naar backend, `/` naar frontend), rate-limiting en authenticatie integraties (bijv. met OAuth2 Proxy).

---

## 🛠️ Installatie

We installeren twee componenten op het cluster.

### 1. NGINX Ingress Controller
Installeer via Helm (de standaard package manager voor Kubernetes):

```bash
helm upgrade --install ingress-nginx ingress-nginx \
  --repo https://kubernetes.github.io/ingress-nginx \
  --namespace ingress-nginx --create-namespace
```

### 2. cert-manager
Installeer cert-manager en de Custom Resource Definitions (CRDs):

```bash
helm upgrade --install cert-manager cert-manager \
  --repo https://charts.jetstack.io \
  --namespace cert-manager --create-namespace \
  --set installCRDs=true
```

Configureer vervolgens een `ClusterIssuer` (wie de certificaten uitgeeft). Voorbeeld voor Let's Encrypt Staging (veilig om mee te testen):

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-staging
spec:
  acme:
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    email: beheer@waterschap.nl
    privateKeySecretRef:
      name: letsencrypt-staging
    solvers:
    - http01:
        ingress:
          class: nginx
```

---

## 🚀 Gebruik

Wanneer de **Builder Agent** een nieuwe webapplicatie bouwt, genereert hij een `Ingress` resource. Door simpelweg een paar regels toe te voegen, regelt het platform de rest.

### Ingress Definitie
Hieronder een voorbeeld hoe een applicatie wordt ontsloten op `mijn-app.druppie.nl` met automatische HTTPS.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: mijn-web-app
  annotations:
    # Vertel cert-manager om een certificaat te regelen via de ClusterIssuer
    cert-manager.io/cluster-issuer: "letsencrypt-staging"
    # Zorg dat HTTP verkeer wordt doorgestuurd naar HTTPS
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - mijn-app.druppie.nl
    secretName: mijn-app-tls  # cert-manager slaat hier het certificaat in op
  rules:
  - host: mijn-app.druppie.nl
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: mijn-app-service
            port:
              number: 80
```

## 🔄 Integratie in Druppie
1.  **Capability Check**: Als een applicatie een gebruikersinterface of API heeft ("public facing"), activeert de **Planner** dit bouwblok.
2.  **Builder Agent**: 
    *   Vraagt domeinnaam instellingen op uit de *Bouwblok Definities*.
    *   Genereert de `Ingress` YAML.
    *   Zorgt dat de applicatie zelf (de Pod) gewoon op poort 80 (HTTP) kan draaien, zonder dat de ontwikkelaar zich zorgen hoeft te maken over SSL-certificaten of complexe netwerkconfiguraties.
3.  **Runtime**: 
    *   De NGINX controller ziet de nieuwe Ingress en past de routing aan.
    *   cert-manager ziet de annotatie, valideert het domein en installeert het certificaat in de achtergrond.
