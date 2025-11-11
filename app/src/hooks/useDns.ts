/**
 * DNS Management Hook
 *
 * Manages DNS configuration for vibespace subdomain routing.
 * Handles DNS setup, status checking, and system DNS configuration.
 *
 * Features:
 * - Check DNS setup status (configured, running, port availability)
 * - Configure system DNS to use bundled DNS server
 * - Start/stop DNS server
 * - Validate DNS resolution
 *
 * @public
 * @see SPEC.md Section 8.1 (DNS Resolution)
 * @see ADR 0007 (DNS Resolution Strategy)
 */

import { useState, useEffect, useCallback } from 'react';
import { invoke } from '@tauri-apps/api/core';

/**
 * DNS setup status
 */
export interface DnsStatus {
  /** Whether DNS is configured in system resolver */
  configured: boolean;
  /** Whether DNS server is running */
  running: boolean;
  /** DNS server port (5353) */
  port: number;
  /** Base domain for subdomains */
  baseDomain: string;
  /** Error message if any */
  error?: string;
}

/**
 * DNS hook return type
 */
export interface UseDnsResult {
  /** Current DNS status */
  status: DnsStatus | null;
  /** Whether status is loading */
  loading: boolean;
  /** Error message if any */
  error: string | null;
  /** Check DNS status */
  checkStatus: () => Promise<void>;
  /** Configure system DNS */
  configureDns: () => Promise<void>;
  /** Remove DNS configuration */
  unconfigureDns: () => Promise<void>;
  /** Start DNS server */
  startDns: () => Promise<void>;
  /** Stop DNS server */
  stopDns: () => Promise<void>;
  /** Test DNS resolution for a hostname */
  testResolution: (hostname: string) => Promise<boolean>;
}

/**
 * Hook for managing DNS configuration
 *
 * @example
 * ```tsx
 * function DnsSetup() {
 *   const { status, configureDns, testResolution } = useDns();
 *
 *   if (!status?.configured) {
 *     return <button onClick={configureDns}>Enable DNS</button>;
 *   }
 *
 *   return <div>DNS configured on port {status.port}</div>;
 * }
 * ```
 */
export function useDns(): UseDnsResult {
  const [status, setStatus] = useState<DnsStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  /**
   * Check DNS setup status
   */
  const checkStatus = useCallback(async () => {
    setLoading(true);
    setError(null);

    try {
      const dnsStatus = await invoke<DnsStatus>('check_dns_status');
      setStatus(dnsStatus);
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to check DNS status';
      setError(errorMessage);
      console.error('Failed to check DNS status:', err);
    } finally {
      setLoading(false);
    }
  }, []);

  /**
   * Configure system DNS to use vibespace DNS server
   */
  const configureDns = useCallback(async () => {
    setLoading(true);
    setError(null);

    try {
      await invoke('configure_dns');
      await checkStatus(); // Refresh status
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to configure DNS';
      setError(errorMessage);
      console.error('Failed to configure DNS:', err);
      throw err; // Re-throw for caller handling
    } finally {
      setLoading(false);
    }
  }, [checkStatus]);

  /**
   * Remove DNS configuration from system resolver
   */
  const unconfigureDns = useCallback(async () => {
    setLoading(true);
    setError(null);

    try {
      await invoke('unconfigure_dns');
      await checkStatus(); // Refresh status
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to unconfigure DNS';
      setError(errorMessage);
      console.error('Failed to unconfigure DNS:', err);
      throw err;
    } finally {
      setLoading(false);
    }
  }, [checkStatus]);

  /**
   * Start DNS server
   */
  const startDns = useCallback(async () => {
    setLoading(true);
    setError(null);

    try {
      await invoke('start_dns');
      await checkStatus(); // Refresh status
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to start DNS server';
      setError(errorMessage);
      console.error('Failed to start DNS server:', err);
      throw err;
    } finally {
      setLoading(false);
    }
  }, [checkStatus]);

  /**
   * Stop DNS server
   */
  const stopDns = useCallback(async () => {
    setLoading(true);
    setError(null);

    try {
      await invoke('stop_dns');
      await checkStatus(); // Refresh status
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to stop DNS server';
      setError(errorMessage);
      console.error('Failed to stop DNS server:', err);
      throw err;
    } finally {
      setLoading(false);
    }
  }, [checkStatus]);

  /**
   * Test DNS resolution for a hostname
   *
   * @param hostname - Hostname to resolve (e.g., "code.myproject.vibe.space")
   * @returns Whether resolution succeeded
   */
  const testResolution = useCallback(async (hostname: string): Promise<boolean> => {
    try {
      const result = await invoke<boolean>('test_dns_resolution', { hostname });
      return result;
    } catch (err) {
      console.error('DNS resolution test failed:', err);
      return false;
    }
  }, []);

  // Check status on mount
  useEffect(() => {
    checkStatus();
  }, [checkStatus]);

  return {
    status,
    loading,
    error,
    checkStatus,
    configureDns,
    unconfigureDns,
    startDns,
    stopDns,
    testResolution,
  };
}
