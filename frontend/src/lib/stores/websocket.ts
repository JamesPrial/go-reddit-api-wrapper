import { writable, type Writable } from 'svelte/store';
import type { WebSocketMessage } from '../api/types';

export enum ConnectionStatus {
  CONNECTING = 'connecting',
  CONNECTED = 'connected',
  DISCONNECTED = 'disconnected',
  ERROR = 'error',
  RECONNECTING = 'reconnecting'
}

interface WebSocketStore {
  status: Writable<ConnectionStatus>;
  error: Writable<string | null>;
  lastMessage: Writable<WebSocketMessage | null>;
  send: (message: any) => void;
  connect: () => void;
  disconnect: () => void;
}

/**
 * Creates a WebSocket store with automatic reconnection
 * Based on research Pattern 1: Basic WebSocket Store
 */
export function createWebSocketStore(url: string): WebSocketStore {
  const status = writable<ConnectionStatus>(ConnectionStatus.DISCONNECTED);
  const error = writable<string | null>(null);
  const lastMessage = writable<WebSocketMessage | null>(null);

  let ws: WebSocket | null = null;
  let reconnectTimeout: ReturnType<typeof setTimeout> | null = null;
  let reconnectAttempts = 0;
  const maxReconnectAttempts = 5;
  const baseReconnectDelay = 1000; // 1 second

  function connect() {
    if (ws && (ws.readyState === WebSocket.CONNECTING || ws.readyState === WebSocket.OPEN)) {
      return;
    }

    try {
      status.set(reconnectAttempts > 0 ? ConnectionStatus.RECONNECTING : ConnectionStatus.CONNECTING);
      error.set(null);

      ws = new WebSocket(url);

      ws.onopen = () => {
        console.log('[WebSocket] Connected');
        status.set(ConnectionStatus.CONNECTED);
        reconnectAttempts = 0;
        error.set(null);
      };

      ws.onmessage = (event) => {
        try {
          const message: WebSocketMessage = JSON.parse(event.data);
          lastMessage.set(message);
        } catch (err) {
          console.error('[WebSocket] Failed to parse message:', err);
          error.set('Failed to parse message');
        }
      };

      ws.onerror = (event) => {
        console.error('[WebSocket] Error:', event);
        status.set(ConnectionStatus.ERROR);
        error.set('WebSocket connection error');
      };

      ws.onclose = (event) => {
        console.log('[WebSocket] Closed:', event.code, event.reason);
        status.set(ConnectionStatus.DISCONNECTED);
        ws = null;

        // Attempt reconnection if not manually closed
        if (event.code !== 1000 && reconnectAttempts < maxReconnectAttempts) {
          const delay = Math.min(baseReconnectDelay * Math.pow(2, reconnectAttempts), 30000);
          console.log(`[WebSocket] Reconnecting in ${delay}ms (attempt ${reconnectAttempts + 1}/${maxReconnectAttempts})`);

          reconnectTimeout = setTimeout(() => {
            reconnectAttempts++;
            connect();
          }, delay);
        } else if (reconnectAttempts >= maxReconnectAttempts) {
          error.set('Max reconnection attempts reached');
        }
      };
    } catch (err) {
      console.error('[WebSocket] Failed to connect:', err);
      status.set(ConnectionStatus.ERROR);
      error.set(err instanceof Error ? err.message : 'Failed to connect');
    }
  }

  function disconnect() {
    if (reconnectTimeout) {
      clearTimeout(reconnectTimeout);
      reconnectTimeout = null;
    }

    reconnectAttempts = 0;

    if (ws) {
      ws.close(1000, 'Client disconnect');
      ws = null;
    }

    status.set(ConnectionStatus.DISCONNECTED);
  }

  function send(message: any) {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(message));
    } else {
      console.warn('[WebSocket] Cannot send message - not connected');
      error.set('Not connected');
    }
  }

  return {
    status,
    error,
    lastMessage,
    send,
    connect,
    disconnect
  };
}
