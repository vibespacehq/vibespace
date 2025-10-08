// Kubernetes Detection Types

export interface KubernetesStatus {
  available: boolean;
  installType?: string;
  version?: string;
  kubeconfigPath?: string;
  error?: string;
  suggestedAction?: 'install_kubernetes' | 'start_kubernetes' | 'check_installation';
}

// TODO(Phase 1): Will be used for type-safe install type checking
export type KubernetesInstallType =
  | 'k3s'
  | 'rancher-desktop'
  | 'k3d'
  | 'minikube'
  | 'docker-desktop'
  | 'unknown';

// Workspace Types
// TODO(Phase 1): Will be used for workspace CRUD operations

export interface Workspace {
  id: string;
  name: string;
  template: string;
  status: 'creating' | 'running' | 'stopped' | 'error';
  createdAt: string;
  url?: string;
}

export interface CreateWorkspaceRequest {
  name: string;
  template: string;
  persistent?: boolean;
}

// Template Types
// TODO(Phase 1): Will be used for template selection

export interface Template {
  id: string;
  name: string;
  description: string;
  image: string;
  tags: string[];
}

// Credential Types
// TODO(Phase 2): Will be used for credential management UI

export interface Credential {
  id: string;
  name: string;
  credType: string;
  createdAt: string;
}

export interface CredentialData {
  name: string;
  credType: string;
  data: Record<string, unknown>;
}

export interface SshKeyPair {
  id: string;
  name: string;
  publicKey: string;
  keyType: 'ed25519' | 'rsa';
  createdAt: string;
}
