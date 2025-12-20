# MCP Catalogus Overview

## 📚 Wat is dit?
De **MCP Catalogus** is een gecureerde lijst van Model Context Protocol (MCP) servers die beschikbaar en goedgekeurd zijn voor gebruik binnen het Druppie platform.

## 🎯 Doel
Het doel van deze catalogus is om ontwikkelaars en architecten snel inzicht te geven in welke integraties "out-of-the-box" beschikbaar zijn. Dit voorkomt dat we het wiel opnieuw uitvinden en zorgt voor standaardisatie.

## 🏗️ Categorieën

### [Microsoft & Azure](./microsoft.md)
MCP servers die specifiek zijn ontworpen voor het Microsoft ecosysteem.
*   **Use Case**: Beheer van Azure resources, toegang tot Office 365 data, integratie met Copilot en Azure AI.
*   **Security**: Maakt zwaar gebruik van Entra ID (Managed Identities) voor authenticatie.

### [Open Source & Generiek](./opensource.md)
Breed inzetbare, community-driven of standaard infrastructuur servers.
*   **Use Case**: Database toegang (Postgres), Git operaties, bestandsmanipulatie, web browsing.
*   **Flexibiliteit**: Vaak eenvoudig in te zetten als container zonder zware dependencies.

## 🔄 Hoe een nieuwe MCP server toevoegen?
1.  **Selectie**: Kies een server uit de [officiële MCP lijst](https://github.com/modelcontextprotocol/servers) of ontwikkel een eigen [MCP Host](../bouwblokken/mcp_server.md).
2.  **Validatie**: Controleer of de server voldoet aan de security eisen (geen hardcoded secrets, containerized).
3.  **Registratie**: Voeg de definitie toe aan een van de bovenstaande Markdown bestanden.
4.  **Deployment**: Rol de server uit op het Kubernetes cluster.
