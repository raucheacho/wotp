import { useCallback, useEffect, useRef, useState } from 'react';

type WsStatus = 'connecting' | 'connected' | 'disconnected';

interface UseWebSocketOptions {
  url: string;
  onMessage?: (data: unknown) => void;
  reconnect?: boolean;
  maxRetries?: number;
}

export function useWebSocket({ url, onMessage, reconnect = true, maxRetries = 10 }: UseWebSocketOptions) {
  const [status, setStatus] = useState<WsStatus>('disconnected');
  const wsRef = useRef<WebSocket | null>(null);
  const retriesRef = useRef(0);
  const timerRef = useRef<any>(null);
  const onMessageRef = useRef(onMessage);
  onMessageRef.current = onMessage;

  const getBackoff = useCallback((attempt: number) => {
    const base = [1000, 2000, 5000, 10000, 30000];
    const delay = base[Math.min(attempt, base.length - 1)];
    // Add jitter
    return delay + Math.random() * 1000;
  }, []);

  const connect = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return;

    // Build absolute WS URL
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = url.startsWith('ws') ? url : `${protocol}//${window.location.host}${url}`;

    setStatus('connecting');

    try {
      const ws = new WebSocket(wsUrl);
      wsRef.current = ws;

      ws.onopen = () => {
        setStatus('connected');
        retriesRef.current = 0;
      };

      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          onMessageRef.current?.(data);
        } catch {
          // ignore non-JSON messages
        }
      };

      ws.onclose = () => {
        setStatus('disconnected');
        wsRef.current = null;

        if (reconnect && retriesRef.current < maxRetries) {
          const delay = getBackoff(retriesRef.current);
          retriesRef.current++;
          timerRef.current = setTimeout(connect, delay);
        }
      };

      ws.onerror = () => {
        ws.close();
      };
    } catch {
      setStatus('disconnected');
      if (reconnect && retriesRef.current < maxRetries) {
        const delay = getBackoff(retriesRef.current);
        retriesRef.current++;
        timerRef.current = setTimeout(connect, delay);
      }
    }
  }, [url, reconnect, maxRetries, getBackoff]);

  const disconnect = useCallback(() => {
    if (timerRef.current) clearTimeout(timerRef.current);
    wsRef.current?.close();
    wsRef.current = null;
    setStatus('disconnected');
  }, []);

  useEffect(() => {
    connect();
    return () => {
      disconnect();
    };
  }, [connect, disconnect]);

  return { status, disconnect, reconnect: connect };
}
