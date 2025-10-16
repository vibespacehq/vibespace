// Kubernetes Detection Types

export interface KubernetesStatus {
  available: boolean;
  installType?: string;
  version?: string;
  kubeconfigPath?: string;
  error?: string;
  suggestedAction?: 'install_kubernetes' | 'start_kubernetes' | 'check_installation';
}

// Cluster Component Types

/**
 * Status of a single cluster component (Knative, Traefik, Registry, BuildKit).
 * Used to display installation status and health in the UI.
 *
 * @public
 * @see Issue #29 for cluster setup implementation
 */
export interface ComponentStatus {
  installed: boolean;
  version?: string;
  healthy: boolean;
  error?: string;
}

/**
 * Status of all required cluster components.
 * Returned by GET /api/v1/cluster/status.
 *
 * @public
 * @see Issue #29 for cluster setup implementation
 */
export interface ClusterComponents {
  knative: ComponentStatus;
  traefik: ComponentStatus;
  registry: ComponentStatus;
  buildkit: ComponentStatus;
}

/**
 * Complete cluster status including components and configuration.
 * Used to determine if cluster setup is needed.
 *
 * @public
 * @see Issue #29 for cluster setup implementation
 */
export interface ClusterStatus {
  healthy: boolean;
  version?: string;
  components: ClusterComponents;
  config?: {
    knativeDomain?: string;
  };
  message?: string;
}

/**
 * Progress update during cluster component installation.
 * Streamed via SSE from POST /api/v1/cluster/setup.
 *
 * @public
 * @see Issue #29 for cluster setup implementation
 */
export interface SetupProgress {
  component: 'knative' | 'traefik' | 'registry' | 'buildkit';
  status: 'pending' | 'installing' | 'done' | 'error';
  message?: string;
  error?: string;
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

/**
 * Represents a Kubernetes context from the user's kubeconfig.
 * Used to allow users to select which cluster to install components to.
 *
 * @public
 * @see Issue #29 for cluster selection implementation
 */
export interface ClusterContext {
  name: string;
  cluster: string;
  user: string;
  is_current: boolean;
  is_local: boolean;
}

// Workspace Types

/**
 * Resource allocations for a workspace (CPU and memory).
 * Maps to Kubernetes resource requests/limits.
 *
 * @public
 */
export interface WorkspaceResources {
  cpu: string;
  memory: string;
}

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
  status: 'creating' | 'starting' | 'running' | 'stopping' | 'stopped' | 'error';
  resources: WorkspaceResources;
  urls: Record<string, string>;
  persistent: boolean;
  created_at: string;
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
  resources?: WorkspaceResources;
  persistent?: boolean;
  github_repo?: string;
  agent?: string;
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
