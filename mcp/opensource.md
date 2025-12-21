# Open Source & Generieke MCP Servers

Een verzameling van breed inzetbare, open-source MCP servers die direct als container (`docker pull`) ingezet kunnen worden binnen het Druppie cluster.

## 🗄️ Data & Storage

### 1. PostgreSQL (Database)
*   **Repo**: `github.com/modelcontextprotocol/servers/tree/main/src/postgres`
*   **Beschrijving**: Geeft de AI veilige, read-only (of gecontroleerde write) toegang tot databases.
*   **Tools**: `query_database`, `get_schema`
*   **Gebruik in Druppie**: Zie [Database Bouwblok](../bouwblokken/database_postgres.md).

### 2. FileSystem (Lokaal/Shared)
*   **Repo**: `github.com/modelcontextprotocol/servers/tree/main/src/filesystem`
*   **Beschrijving**: Toegang tot specifieke mappen op een Persistent Volume. Handig voor het lezen van logs of rapporten.
*   **Configuratie**: Draait als sidecar of met een shared volume mount.

### 3. SQLite
*   **Repo**: `github.com/modelcontextprotocol/servers/tree/main/src/sqlite`
*   **Beschrijving**: Lichtgewicht database toegang, ideaal voor tijdelijke analyse agents.

## 🛠️ Development Tools

### 4. Git
*   **Repo**: `github.com/modelcontextprotocol/servers/tree/main/src/git`
*   **Beschrijving**: Stelt de AI in staat om code te lezen, diffs te maken en branches te beheren.
*   **Integratie**: Werkt samen met onze [Gitea](../bouwblokken/git_gitea.md) instantie.

### 5. GitHub / GitLab
*   **Repo**: `github.com/modelcontextprotocol/servers/tree/main/src/github`
*   **Beschrijving**: Beheer van Issues, PRs en Releases.
*   **Tools**: `create_issue`, `merge_pr`, `search_code`

## 🌐 Web & Search

### 6. Puppeteer / Playwright (Web Browser)
*   **Repo**: `github.com/modelcontextprotocol/servers` (community)
*   **Beschrijving**: Stelt de AI in staat om websites te bezoeken, screenshots te maken en content te scrapen (bijv. voor onderzoek).
*   **Security**: Draait altijd in een geïsoleerde, tijdelijke container ("Sandbox").

### 7. Brave Search
*   **Repo**: `github.com/modelcontextprotocol/servers/tree/main/src/brave-search`
*   **Beschrijving**: Uitvoeren van web-zoekopdrachten via de Brave Search API (privacy-vriendelijk).

## 🚀 Advanced Capabilities (AI Stack Upgrade)

### 8. Graphiti (Temporal Knowledge Graph)
*   **Repo**: `getzep/graphiti`
*   **Beschrijving**: Voegt een dynamisch geheugen toe aan de AI. Slaat entiteiten en relaties op in een grafiek die verandert in de tijd (Temporal).
*   **Use Case**: Langlopende context onthouden van projectbeslissingen en relaties tussen stakeholders.

### 9. Opik (Observability)
*   **Repo**: `comet-ml/opik`
*   **Beschrijving**: Observability laag specifiek voor LLM-applicaties. Traceert latency, kosten en de "gedachtegang" (chains) van de AI.
*   **Use Case**: Debuggen waarom een Agent een bepaalde keuze heeft gemaakt in de Planner.

### 10. Jupyter MCP (Interactive Notebooks)
*   **Repo**: `qhdwight/jupyter-mcp-server`
*   **Beschrijving**: Laat de AI direct code uitvoeren, plotten en analyseren in Jupyter Notebooks.
*   **Use Case**: Data Science taken en het visualiseren van metrics (bijv. drone vlucht logs) zonder code te hoeven deployen.
