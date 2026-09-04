"use client";
import { useCallback, useEffect, useRef, useState } from "react";
export class ApiError extends Error {
  constructor(
    message: string,
    public code: string,
    public status: number,
    public details?: unknown,
  ) {
    super(message);
  }
}
export async function api<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`/v1${path}`, {
    credentials: "same-origin",
    ...options,
    headers: { "Content-Type": "application/json", ...options?.headers },
  });
  const body = await response.json().catch(() => null);
  if (!response.ok || body?.error)
    throw new ApiError(
      body?.error?.message ||
        `The service returned ${response.status}. Please try again.`,
      body?.error?.code || "service_unavailable",
      response.status,
      body?.error?.details,
    );
  return body as T;
}
export function useResource<T>(path: string | null, interval = 0) {
  const [record, setRecord] = useState<{ path: string; value: T }>();
  const [error, setError] = useState<Error>();
  const [loading, setLoading] = useState(true);
  const [revision, setRevision] = useState(0);
  const refresh = useCallback(() => setRevision((v) => v + 1), []);
  useEffect(() => {
    if (!path) {
      setLoading(false);
      setRecord(undefined);
      setError(undefined);
      return;
    }
    let active = true;
    let timer: ReturnType<typeof setTimeout>;
    const controller = new AbortController();
    const read = async () => {
      try {
        const value = await api<T>(path, { signal: controller.signal });
        if (active) {
          setRecord({ path, value });
          setError(undefined);
        }
      } catch (e) {
        if (active && (e as Error).name !== "AbortError") setError(e as Error);
      } finally {
        if (active) {
          setLoading(false);
          if (interval) timer = setTimeout(read, interval);
        }
      }
    };
    setLoading(true);
    read();
    return () => {
      active = false;
      controller.abort();
      clearTimeout(timer);
    };
  }, [path, interval, revision]);
  return {
    data: record?.path === path ? record.value : undefined,
    error,
    loading,
    refresh,
  };
}
export function useAction() {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<Error>();
  const retry = useRef<{ signature: string; key: string } | null>(null);
  const run = async <T>(
    path: string,
    body: unknown = {},
  ): Promise<T | undefined> => {
    const serialized = JSON.stringify(body);
    const signature = path + serialized;
    if (retry.current?.signature !== signature)
      retry.current = { signature, key: crypto.randomUUID() };
    setBusy(true);
    setError(undefined);
    try {
      const result = await api<T>(path, {
        method: "POST",
        body: serialized,
        headers: { "Idempotency-Key": retry.current.key },
      });
      retry.current = null;
      return result;
    } catch (e) {
      setError(e as Error);
      return undefined;
    } finally {
      setBusy(false);
    }
  };
  return { run, busy, error, clearError: () => setError(undefined) };
}
