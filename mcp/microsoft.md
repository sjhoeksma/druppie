# Microsoft & Azure MCP Servers

Deze lijst bevat MCP servers die integreren met het Microsoft ecosysteem, specifiek gericht op Azure en Office 365, geschikt voor gebruik binnen Druppie.

## ☁️ Azure Beheer & DevOps

### 1. Azure Resource Manager (ARM)
*   **Beschrijving**: Biedt toegang tot het lezen en beheren van Azure resources via de ARM API.
*   **Capabilities**: `resources`, `prompts`
*   **Tools**: `list_resources`, `get_resource_metrics`, `restart_vm`
*   **Bron/Repo**: `github.com/microsoft/mcp-azure-arm` (Hypothetisch/Custom implementation)

### 2. Azure DevOps
*   **Beschrijving**: Interactie met Azure Boards (work items) en Pipelines.
*   **Capabilities**: `tools`
*   **Tools**: `get_work_item`, `create_bug`, `list_pipelines`
*   **Status**: In ontwikkeling voor Druppie.

## 🏢 Productivity (M365)

### 3. Microsoft Graph
*   **Beschrijving**: De gateway naar data in Microsoft 365 (Email, Calendar, Teams, SharePoint).
*   **Capabilities**: `resources`, `tools`
*   **Tools**: `search_sharepoint`, `get_calendar_events`, `send_teams_message`
*   **Security**: Vereist specifieke Entra ID scopes (`Files.Read`, `Calendars.Read`).

## 🧠 AI & Data

### 4. Azure AI Search (Knowledge Base)
*   **Beschrijving**: Verbindt de RAG (Retrieval Augmented Generation) kennisbank aan het model via MCP.
*   **Capabilities**: `resources` (leest documenten als resources)
*   **Beschikbaarheid**: Geïntegreerd in het [Knowledge Bot](../bouwblokken/knowledge_bot.md) bouwblok.

### 5. Microsoft Copilot (M365) Agent
*   **Beschrijving**: Stelt Druppie in staat om te praten met de bestaande Copilot agents binnen de tenant. Hierdoor kan Druppie vragen stellen aan Copilot ("Wat is de samenvatting van deze meeting?") of acties delegeren.
*   **Capabilities**: `prompts` (handoff), `tools` (invoke agent)
*   **Integration**: Gebruikt de Copilot Studio extensies framework.
