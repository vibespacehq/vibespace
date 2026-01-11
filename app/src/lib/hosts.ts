/**
 * Hosts file management utilities
 *
 * Manages /etc/hosts entries for vibespace DNS resolution.
 * Uses Tauri's update_hosts_file command which requires sudo.
 *
 * @see ADR 0007 (DNS Resolution Strategy)
 */

import { invoke } from '@tauri-apps/api/core';

export interface HostEntry {
  ip: string;
  hostname: string;
}

const BASE_DOMAIN = 'vibe.space';
const LOCALHOST_IP = '127.0.0.1';

/**
 * Updates /etc/hosts with vibespace entries.
 * This replaces ALL vibespace-managed entries with the provided list.
 *
 * @param entries - Array of host entries to add
 * @throws Error if update fails (e.g., user cancels sudo prompt)
 */
export async function updateHostsFile(entries: HostEntry[]): Promise<void> {
  try {
    await invoke('update_hosts_file', { entries });
    console.log('[hosts] Updated /etc/hosts with', entries.length, 'entries');
  } catch (err) {
    console.error('[hosts] Failed to update /etc/hosts:', err);
    throw new Error(
      err instanceof Error ? err.message : 'Failed to update hosts file'
    );
  }
}

/**
 * Generates host entries for a vibespace project.
 * Creates entries for the main domain and common port subdomains.
 *
 * @param projectName - DNS-friendly project name (e.g., "swift-fox")
 * @returns Array of host entries for this vibespace
 */
export function generateHostEntries(projectName: string): HostEntry[] {
  return [
    // Main domain: {project}.vibe.space
    { ip: LOCALHOST_IP, hostname: `${projectName}.${BASE_DOMAIN}` },
    // Common development ports as subdomains
    { ip: LOCALHOST_IP, hostname: `3000.${projectName}.${BASE_DOMAIN}` },
    { ip: LOCALHOST_IP, hostname: `3001.${projectName}.${BASE_DOMAIN}` },
    { ip: LOCALHOST_IP, hostname: `4000.${projectName}.${BASE_DOMAIN}` },
    { ip: LOCALHOST_IP, hostname: `5000.${projectName}.${BASE_DOMAIN}` },
    { ip: LOCALHOST_IP, hostname: `5173.${projectName}.${BASE_DOMAIN}` },
    { ip: LOCALHOST_IP, hostname: `8000.${projectName}.${BASE_DOMAIN}` },
    { ip: LOCALHOST_IP, hostname: `8080.${projectName}.${BASE_DOMAIN}` },
    { ip: LOCALHOST_IP, hostname: `8081.${projectName}.${BASE_DOMAIN}` },
  ];
}

/**
 * Syncs /etc/hosts with all active vibespace project names.
 * Generates entries for each project and updates the hosts file.
 *
 * @param projectNames - Array of active vibespace project names
 */
export async function syncHostsFile(projectNames: string[]): Promise<void> {
  const allEntries: HostEntry[] = [];

  for (const projectName of projectNames) {
    if (projectName) {
      allEntries.push(...generateHostEntries(projectName));
    }
  }

  await updateHostsFile(allEntries);
}
