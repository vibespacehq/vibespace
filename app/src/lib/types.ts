// Kubernetes Detection Types

export interface KubernetesStatus {
  available: boolean;
  installType?: string;
  version?: string;
  kubeconfigPath?: string;
  error?: string;
  suggestedAction?: 'install_kubernetes' | 'start_kubernetes' | 'check_installation';
}

/**
 * Type-safe enumeration of supported Kubernetes installation types.
 * Used to determine which Kubernetes distribution is running on the system.
 *
 * @public
 * @see SPEC.md Section 4.3.1 for detection logic details
 */
export type KubernetesInstallType =
  | 'k3s'
  | 'rancher-desktop'
  | 'k3d'
  | 'minikube'
  | 'docker-desktop'
  | 'unknown';

// Workspace Types
// TODO(Phase 1): Will be used for workspace CRUD operations

/**
 * Represents a workspace instance running in the Kubernetes cluster.
 * Each workspace is an isolated development environment with code-server and project-specific configuration.
 *
 * @public
 * @see SPEC.md Section 5 for complete workspace specifications
 */
export interface Workspace {
  id: string;
  name: string;
  template: string;
  status: 'creating' | 'running' | 'stopped' | 'error';
  createdAt: string;
  url?: string;
}

/**
 * Request payload for creating a new workspace.
 * Defines the minimum required configuration to spin up a development environment.
 *
 * @public
 * @see SPEC.md Section 6.2 for API endpoint details
 */
export interface CreateWorkspaceRequest {
  name: string;
  template: string;
  persistent?: boolean;
}

// Template Types
// TODO(Phase 1): Will be used for template selection

/**
 * Represents a workspace template (e.g., Next.js, Vue, Jupyter).
 * Templates define the base image and pre-installed tools for a workspace.
 *
 * @public
 * @see SPEC.md Section 9 for template system architecture
 */
export interface Template {
  id: string;
  name: string;
  description: string;
  image: string;
  tags: string[];
}

// Credential Types
// TODO(Phase 2): Will be used for credential management UI

/**
 * Represents a stored credential (AI agent API key, SSH key, Git config, etc.).
 * Credentials are encrypted at rest using AES-256 and stored in OS keychain.
 *
 * @public
 * @see SPEC.md Section 7.1 for credential management details
 */
export interface Credential {
  id: string;
  name: string;
  credType: string;
  createdAt: string;
}

/**
 * Payload for creating or updating a credential.
 * Contains credential type, name, and provider-specific data.
 *
 * @public
 * @see SPEC.md Section 7.1 for supported credential types
 */
export interface CredentialData {
  name: string;
  credType: string;
  data: Record<string, unknown>;
}

/**
 * Represents an SSH key pair (public + private) for Git authentication.
 * Private keys are stored encrypted, only public keys are returned in API responses.
 *
 * @public
 * @see SPEC.md Section 7.1.3 for SSH key generation workflow
 */
export interface SshKeyPair {
  id: string;
  name: string;
  publicKey: string;
  keyType: 'ed25519' | 'rsa';
  createdAt: string;
}
