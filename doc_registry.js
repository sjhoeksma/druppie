// Documentation Registry
// This file acts as a simple database for the documentation files.
const DOC_REGISTRY = {
    "bouwblokken": [
        { name: "Overview", path: "bouwblokken/overview.md" },
        { name: "Bouwblok Definities", path: "bouwblokken/bouwblok_definities.md" },
        { name: "CI/CD Pipeline (Tekton)", path: "bouwblokken/ci_cd_tekton.md" },
        { name: "Database (PostgreSQL)", path: "bouwblokken/database_postgres.md" },
        { name: "Geo-Database (PostGIS)", path: "bouwblokken/database_postgis.md" },
        { name: "Webserver & Certificaten (Ingress)", path: "bouwblokken/webserver_ingress.md" },
        { name: "GitOps & State (Flux)", path: "bouwblokken/gitops_flux.md" },
        { name: "Observability (PLG Stack)", path: "bouwblokken/observability_plg.md" },
        { name: "Git & Versiebeheer (Gitea)", path: "bouwblokken/git_gitea.md" },
        { name: "Traceability (Tempo & OTEL)", path: "bouwblokken/traceability_otel.md" },
        { name: "IAM (Keycloak & Azure AD)", path: "bouwblokken/iam_keycloak.md" },
        { name: "MCP Server Host", path: "bouwblokken/mcp_server.md" },
        { name: "Build Plane", path: "bouwblokken/build_plane.md" },
        { name: "Compliance Layer", path: "bouwblokken/compliance_layer.md" },
        { name: "Druppie Core", path: "bouwblokken/druppie_core.md" },
        { name: "Druppie UI", path: "bouwblokken/druppie_ui.md" },
        { name: "Knowledge Bot", path: "bouwblokken/knowledge_bot.md" },
        { name: "Mens in de Loop", path: "bouwblokken/mens_in_de_loop.md" },
        { name: "Policy Engine", path: "bouwblokken/policy_engine.md" },
        { name: "Runtime", path: "bouwblokken/runtime.md" },
        { name: "Traceability DB", path: "bouwblokken/traceability_db.md" }
    ],
    "skills": [
        { name: "Overview", path: "skills/overview.md" },
        { name: "Mermaid Diagrams", path: "skills/mermaid.md" },
        { name: "Kubernetes Ops", path: "skills/kubernets.md" },
        { name: "Test Skill", path: "skills/test.md" },
        { name: "Node.js Skill", path: "skills/nodejs.md" },
        { name: "Python Skill", path: "skills/python.md" },
        { name: "Architect Skill", path: "skills/architect.md" },
        { name: "Business Analyst Skill", path: "skills/business_analyst.md" }
    ],
    "build_plane": [
        { name: "Overview", path: "build_plane/overview.md" },
        { name: "Builder Agent", path: "build_plane/builder_agent.md" },
        { name: "Foundry", path: "build_plane/foundry.md" }
    ],
    "runtime": [
        { name: "Overview", path: "runtime/overview.md" },
        { name: "Runtime Info", path: "runtime/runtime.md" },
        { name: "Role Based Access Control (RBAC)", path: "runtime/rbac.md" },
        { name: "MCP Interface", path: "runtime/mcp_interface.md" },
        { name: "Dynamic Slot", path: "runtime/dynamic_slot.md" },
        { name: "Git", path: "runtime/git.md" }
    ],
    "compliance": [
        { name: "Overview", path: "compliance/overview.md" },
        { name: "BIO & NIS2", path: "compliance/bio4_nis2.md" },
        { name: "IAM", path: "compliance/iam.md" }
    ],
    "mcp_catalog": [
        { name: "Overview", path: "mcp/overview.md" },
        { name: "Microsoft & Azure", path: "mcp/microsoft.md" },
        { name: "Open Source Tools", path: "mcp/opensource.md" }
    ],
    "general": [
        { name: "Project Readme", path: "README.md" },
        { name: "Het Verhaal Druppie", path: "story/story.md" }
    ]
};
