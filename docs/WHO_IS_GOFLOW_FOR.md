# Who Is Goflow For?

Goflow is a local-first workflow runner for developers, AI agents, homelabs, edge devices, and small internal systems.

It is for people who want visual workflow composition and several controlled execution interfaces without operating a larger automation platform or giving an agent direct access to databases, credentials, and infrastructure.

## Primary Scenario

A developer or operator runs a small set of trusted automations on one machine or private server. Workflows are built visually, stored locally, and invoked through the UI, REST API, CLI, cron, webhooks, or MCP.

```text
UI / REST / CLI / MCP / Webhook / Cron
                  |
              Goflow
       approved workflow boundary
                  |
       APIs / data / notifications
```

The workflow—not the caller—defines which credentials and capabilities are available. Scoped tokens and workflow allowlists let automation clients run approved workflows without receiving direct access to every underlying system.

## Good Fit

Goflow is a good fit when:

- a single binary and embedded SQLite simplify deployment;
- workflows run on a trusted machine or private server;
- developers are comfortable reviewing workflow definitions;
- a focused node set is sufficient;
- AI agents should invoke approved tools rather than arbitrary infrastructure;
- local ownership matters more than a hosted integration marketplace.

## Not a Fit

Do not choose Goflow when you need:

- a large managed connector ecosystem;
- non-technical business users to administer complex automation;
- multi-user workspaces, SSO, or enterprise RBAC;
- hostile multi-tenant workflow execution;
- a sandbox that makes untrusted workflows safe;
- distributed queues, high availability, or large-scale orchestration;
- a drop-in replacement for n8n, Zapier, Make, or Dify.

## Capability Boundary

A Goflow workflow can perform powerful actions. Depending on its nodes and credentials, it may:

- call arbitrary HTTP endpoints, including private-network services;
- connect to databases;
- send messages and email;
- execute JavaScript in the embedded runtime;
- run commands on remote hosts through SSH;
- run local Git operations and modify repositories;
- pass data to external AI providers.

Treat workflow creation and editing as administrator-level capabilities. Scoped run tokens restrict which approved workflows a client may invoke; they do not make a dangerous workflow safe.

## Current Product Rule

Goflow is maintained as a flagship open-source engineering project. New nodes, protocols, and integrations should be added only when they support a demonstrated workflow or repeated user demand.

Catena, MCP, CLI, and plugins are integration surfaces—not reasons to expand Goflow into a general-purpose platform.
