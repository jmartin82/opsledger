import React, { createContext, useCallback, useContext, useRef, useState } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useSSE, SSEEventType } from '@/hooks/useSSE';

type SSECallback = (data: unknown) => void;

interface LiveContextValue {
  connected: boolean;
  subscribe: (type: SSEEventType, callback: SSECallback) => () => void;
}

const LiveContext = createContext<LiveContextValue | null>(null);

export const LiveProvider = ({ children }: { children: React.ReactNode }) => {
  const { isAuthenticated } = useAuth();
  const [connected, setConnected] = useState(false);

  // Internal registry: event type → set of callbacks
  // Using a ref so subscribe/dispatch don't cause re-renders when the registry changes.
  const registryRef = useRef<Map<SSEEventType, Set<SSECallback>>>(new Map());

  const subscribe = useCallback((type: SSEEventType, callback: SSECallback): (() => void) => {
    const registry = registryRef.current;
    if (!registry.has(type)) {
      registry.set(type, new Set());
    }
    registry.get(type)!.add(callback);

    return () => {
      registry.get(type)?.delete(callback);
    };
  }, []);

  useSSE({
    enabled: isAuthenticated,
    callbacks: {
      onConnected: () => setConnected(true),
      onDisconnected: () => setConnected(false),
      onEvent: (type, data) => {
        const handlers = registryRef.current.get(type);
        if (handlers) {
          handlers.forEach(cb => cb(data));
        }
      },
    },
  });

  return (
    <LiveContext.Provider value={{ connected, subscribe }}>
      {children}
    </LiveContext.Provider>
  );
};

export const useLive = (): LiveContextValue => {
  const ctx = useContext(LiveContext);
  if (!ctx) throw new Error('useLive must be used within LiveProvider');
  return ctx;
};
