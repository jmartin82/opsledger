import { useEffect, useRef } from 'react';
import { getToken } from '@/lib/api';

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8081';

export type SSEEventType = 'change.created' | 'change.updated' | 'change.deleted';

export interface SSECallbacks {
  onEvent: (type: SSEEventType, data: unknown) => void;
  onConnected: () => void;
  onDisconnected: () => void;
}

interface UseSSEOptions {
  enabled: boolean;
  callbacks: SSECallbacks;
}

/**
 * useSSE connects to GET /api/events via fetch + ReadableStream.
 * JWT is passed as ?token= so the browser's fetch (or EventSource fallback)
 * can authenticate without a custom header restriction.
 *
 * Reconnects with exponential backoff: 1s → 2s → 4s … capped at 30s.
 * Resets backoff on a successful connection that receives at least one chunk.
 */
export function useSSE({ enabled, callbacks }: UseSSEOptions): void {
  // Store callbacks in refs so backoff/reconnect loops always see the latest
  // versions without needing to be in the dependency array.
  const callbacksRef = useRef(callbacks);
  callbacksRef.current = callbacks;

  useEffect(() => {
    if (!enabled) return;

    let aborted = false;
    const abortController = new AbortController();
    let backoff = 1000; // ms

    const connect = async () => {
      const token = getToken();
      if (!token) return;

      const url = `${API_URL}/api/events?token=${encodeURIComponent(token)}`;

      try {
        const response = await fetch(url, {
          signal: abortController.signal,
          headers: { Accept: 'text/event-stream' },
        });

        if (!response.ok || !response.body) {
          throw new Error(`SSE connection failed: ${response.status}`);
        }

        // Connection established — notify and reset backoff
        callbacksRef.current.onConnected();
        backoff = 1000;

        const reader = response.body.getReader();
        const decoder = new TextDecoder();

        // Accumulate partial lines across chunks
        let buffer = '';
        let currentEventType: string | null = null;
        let currentData: string | null = null;

        while (!aborted) {
          const { done, value } = await reader.read();
          if (done) break;

          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split('\n');
          // Keep any incomplete final line in the buffer
          buffer = lines.pop() ?? '';

          for (const line of lines) {
            if (line.startsWith('event:')) {
              currentEventType = line.slice('event:'.length).trim();
            } else if (line.startsWith('data:')) {
              currentData = line.slice('data:'.length).trim();
            } else if (line === '') {
              // Empty line = end of event
              if (currentEventType && currentData) {
                try {
                  const parsed = JSON.parse(currentData);
                  callbacksRef.current.onEvent(
                    currentEventType as SSEEventType,
                    parsed,
                  );
                } catch {
                  // Malformed JSON — ignore
                }
              }
              currentEventType = null;
              currentData = null;
            }
            // Lines starting with ':' are comments/keepalives — ignore
          }
        }
      } catch (err) {
        if (aborted) return;
        // Swallow AbortError — that's our clean unmount signal
        if (err instanceof DOMException && err.name === 'AbortError') return;
      }

      if (!aborted) {
        // Connection dropped — notify and schedule reconnect
        callbacksRef.current.onDisconnected();
        const delay = backoff;
        backoff = Math.min(backoff * 2, 30_000);
        await new Promise(resolve => setTimeout(resolve, delay));
        if (!aborted) connect();
      }
    };

    connect();

    return () => {
      aborted = true;
      abortController.abort();
      callbacksRef.current.onDisconnected();
    };
  }, [enabled]);
}
