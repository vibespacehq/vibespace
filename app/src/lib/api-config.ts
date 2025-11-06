/**
 * API Configuration
 * Centralized API endpoint configuration for the application.
 */

const API_BASE_URL = (import.meta as { env?: { VITE_API_URL?: string } }).env?.VITE_API_URL || 'http://localhost:8090';

/**
 * API endpoints for cluster and vibespace management
 */
export const API_ENDPOINTS = {
  // Cluster endpoints
  clusterContexts: `${API_BASE_URL}/api/v1/cluster/contexts`,
  clusterContextSwitch: (contextName: string) =>
    `${API_BASE_URL}/api/v1/cluster/contexts/${contextName}/switch`,
  clusterStatus: `${API_BASE_URL}/api/v1/cluster/status`,
  clusterSetup: `${API_BASE_URL}/api/v1/cluster/setup`,

  // Vibespace endpoints
  vibespaces: `${API_BASE_URL}/api/v1/vibespaces`,
} as const;

/**
 * Fetch with error handling
 * @param url - The URL to fetch
 * @param options - Fetch options
 * @returns Response data
 * @throws {Error} With detailed error message
 */
export async function apiFetch<T>(url: string, options?: RequestInit): Promise<T> {
  try {
    const response = await fetch(url, options);

    if (!response.ok) {
      let errorMessage = `HTTP ${response.status}`;
      try {
        const errorData = await response.json();
        errorMessage = errorData.message || errorData.error || errorMessage;
      } catch {
        // If error response is not JSON, use status text
        errorMessage = response.statusText || errorMessage;
      }
      throw new Error(errorMessage);
    }

    return await response.json();
  } catch (err) {
    if (err instanceof Error) {
      throw err;
    }
    throw new Error('Network request failed');
  }
}
