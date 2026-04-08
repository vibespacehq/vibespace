# Vault

The vault is vibespace's built-in secret manager. It stores API keys, tokens, and credentials that agents need at runtime.

## How it works

1. An agent needs an API key (e.g., `STRIPE_SECRET_KEY`)
2. It sends a secret request via the comms server with a reason ("need to call Stripe API")
3. A notification appears in the app's approval ticker
4. You paste the value and click Save
5. The secret is stored in the vault and injected into the agent's environment

Agents can also request integrations — packages or services that require an environment variable (e.g., "install `@sendgrid/mail` which needs `SENDGRID_API_KEY`").

## Storage

Secrets are stored in two tiers:

### Global vault
Location: `~/.vibespace/vault/secrets.json`

Global secrets are shared across all workspaces. When saving a secret, check "All" to make it global. Useful for keys you use in every project (e.g., GitHub tokens, cloud provider credentials).

### Local secrets
Location: `~/.vibespace/data/<id>/.secrets.json`

Local secrets are scoped to a single workspace. They're only available to agents in that specific vibespace.

## Access control

A `policy.json` file controls which agents can access which secrets. Only agents in the allow list for a given secret can retrieve it. This prevents agents from accessing secrets they don't need.

## Loading secrets

When an agent starts, secrets from both the global vault and local store are merged and injected as environment variables. Agents can also load secrets on demand during a session.

## Integrations

The vault also manages integrations — third-party services and packages that agents request. When an agent asks for an integration:

1. The request appears in the notification ticker with the package name and required environment variable
2. You paste the API key
3. The secret is stored and the agent can proceed with the installation

You can view and manage all secrets and integrations from the Vault section in the sidebar.
