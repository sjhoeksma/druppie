---
id: plan-fs-template
name: plan-fs-template
command: npx
args:
  - "-y"
  - "@modelcontextprotocol/server-filesystem"
  - "./.druppie/plans/{{plan_id}}"
transport: stdio
---
# Plan Filesystem Template

This definition acts as a template for per-plan filesystem access.
When a plan starts, Druppie looks for this template.
It creates a new ephemeral MCP server named `plan-<PLAN-ID>-fs`,
substituting `{{plan_id}}` in the arguments.
This allows agents to access the specific directory for the current plan.
