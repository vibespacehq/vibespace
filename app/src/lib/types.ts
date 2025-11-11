// Kubernetes Status Types (Bundled Approach)

/**
 * Status of bundled Kubernetes runtime (Colima on macOS, k3s on Linux).
 * Returned by get_kubernetes_status Tauri command.
 *
 * @public
 * @see ADR 0006 for bundled Kubernetes architecture
 */
export interface KubernetesStatus {
  installed: boolean;
  running: boolean;
  version?: string;
  is_external: boolean;
  error?: string;
  suggested_action?: 'install' | 'start' | 'restart' | 'start_external';
}

/**
 * Progress update during Kubernetes installation.
 * Emitted via 'install-progress' event from install_kubernetes command.
 *
 * @public
 */
export interface InstallProgress {
  stage: 'extracting' | 'installing' | 'starting_vm' | 'starting_k3s' | 'verifying' | 'complete' | 'error';
  progress: number; // 0-100
  message: string;
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

// Vibespace Types

/**
 * Resource allocations for a vibespace (CPU and memory).
 * Maps to Kubernetes resource requests/limits.
 *
 * @public
 */
export interface VibespaceResources {
  cpu: string;
  memory: string;
}

/**
 * Represents a vibespace instance running in the Kubernetes cluster.
 * Each vibespace is an isolated development environment with code-server and project-specific configuration.
 *
 * @public
 * @see SPEC.md Section 5 for complete vibespace specifications
 */
export interface Vibespace {
  id: string;
  name: string;
  project_name?: string; // DNS-friendly name (e.g., "brave-eagle-7421") - added in Knative migration
  template: string;
  status: 'creating' | 'starting' | 'running' | 'stopping' | 'stopped' | 'deleting' | 'error';
  resources: VibespaceResources;
  urls: Record<string, string>; // DNS URLs: {code: "http://code.{project}.vibe.space", preview: ..., prod: ...}
  persistent: boolean;
  created_at: string;
}

/**
 * Request payload for creating a new vibespace.
 * Defines the minimum required configuration to spin up a development environment.
 *
 * @public
 * @see SPEC.md Section 6.2 for API endpoint details
 */
export interface CreateVibespaceRequest {
  name: string;
  template: string;
  resources?: VibespaceResources;
  persistent?: boolean;
  github_repo?: string;
  agent?: string;
}

// Template Types
// TODO(Phase 1): Will be used for template selection

/**
 * Represents a vibespace template (e.g., Next.js, Vue, Jupyter).
 * Templates define the base image and pre-installed tools for a vibespace.
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
