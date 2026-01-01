---
id: util-file-reader
name: File Reader Utility
type: tool
version: 1.0.0
auth_group: []
description: "Utility to read the content of uploaded files within the active plan context."
capabilities: ["read_file", "data_extraction"]
inputs:
  - "filename"
outputs:
  - "text_content"
---

# Bouwblok: File Reader Utility

## 🎯 Doelstelling
Het lezen van de inhoud van geuploade bestanden (zoals text, markdown, csv) zodat agents deze kunnen analyseren.

## 🏗️ Architectuur
- **Inputs**: Bestandsnaam (moet geupload zijn in het huidige plan).
- **Features**: 
    - **Safe Read**: Leest alleen vanuit de `.druppie/files/<plan-id>` sandbox.
    - **Format Agnostic**: Geeft raw text terug.
- **Implementation**: Go-based Executor.

## 🛠️ Integratie
- Wordt gebruikt door **Business Analyst**, **Data Scientist** en **Planner-Agents**.
- Directe output naar de context van de agent.
