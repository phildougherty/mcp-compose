import { useEffect, useRef, useState, useCallback } from 'react';

/**
 * Custom hook for WebSocket connection management
 *
 * @param {string} url - WebSocket URL
 * @param {Object} options - Configuration options
 * @param {boolean} options.autoConnect - Auto-connect on mount (default: true)
 * @param {number} options.reconnectDelay - Delay between reconnection attempts in ms (default: 3000)
 * @param {number} options.maxReconnectAttempts - Maximum reconnection attempts (default: 5)
 * @param {Function} options.onMessage - Message handler callback
 * @param {Function} options.onOpen - Open event callback
 * @param {Function} options.onClose - Close event callback
 * @param {Function} options.onError - Error event callback
 * @returns {Object} WebSocket state and control functions
 */
export function useWebSocket(url, options = {}) {
  const {
    autoConnect = true,
    reconnectDelay = 3000,
    maxReconnectAttempts = 5,
    onMessage,
    onOpen,
    onClose,
    onError
  } = options;

  const [isConnected, setIsConnected] = useState(false);
  const [lastMessage, setLastMessage] = useState(null);
  const [connectionError, setConnectionError] = useState(null);
  const [reconnectAttempts, setReconnectAttempts] = useState(0);

  const wsRef = useRef(null);
  const reconnectTimeoutRef = useRef(null);
  const shouldReconnectRef = useRef(true);
  const mountedRef = useRef(true);
  const reconnectAttemptsRef = useRef(0);

  const onMessageRef = useRef(onMessage);
  const onOpenRef = useRef(onOpen);
  const onCloseRef = useRef(onClose);
  const onErrorRef = useRef(onError);

  useEffect(() => {
    onMessageRef.current = onMessage;
    onOpenRef.current = onOpen;
    onCloseRef.current = onClose;
    onErrorRef.current = onError;
  }, [onMessage, onOpen, onClose, onError]);

  const connect = useCallback(() => {
    if (!url) {
      console.log('[useWebSocket] Cannot connect: URL is null/undefined');

      return;
    }

    if (wsRef.current?.readyState === WebSocket.OPEN) {
      console.log('[useWebSocket] Already connected, skipping');

      return;
    }

    try {
      console.log('[useWebSocket] Creating WebSocket connection to:', url);
      const ws = new WebSocket(url);

      ws.onopen = (event) => {
        if (!mountedRef.current) return;

        console.log('[useWebSocket] WebSocket opened successfully');
        setIsConnected(true);
        setConnectionError(null);
        setReconnectAttempts(0);
        reconnectAttemptsRef.current = 0;

        if (onOpenRef.current) {
          onOpenRef.current(event);
        }
      };

      ws.onmessage = (event) => {
        if (!mountedRef.current) return;

        let data = event.data;

        try {
          data = JSON.parse(event.data);
        } catch (e) {
        }

        setLastMessage({ data, timestamp: Date.now() });

        if (onMessageRef.current) {
          onMessageRef.current(data, event);
        }
      };

      ws.onerror = (event) => {
        if (!mountedRef.current) return;

        console.error('[useWebSocket] WebSocket error:', event);
        const error = new Error('WebSocket error');
        setConnectionError(error);

        if (onErrorRef.current) {
          onErrorRef.current(error, event);
        }
      };

      ws.onclose = (event) => {
        if (!mountedRef.current) return;

        console.log('[useWebSocket] WebSocket closed - code:', event.code, 'reason:', event.reason);
        setIsConnected(false);

        if (onCloseRef.current) {
          onCloseRef.current(event);
        }

        if (shouldReconnectRef.current && reconnectAttemptsRef.current < maxReconnectAttempts) {
          const isSessionNotFound = event.code === 1006 || (event.code === 1000 && event.reason?.includes('Session not found'));
          const delay = isSessionNotFound
            ? Math.min(1000 * Math.pow(2, reconnectAttemptsRef.current), 5000)
            : reconnectDelay;

          console.log('[useWebSocket] Scheduling reconnect attempt', reconnectAttemptsRef.current + 1, 'delay:', delay);
          reconnectTimeoutRef.current = setTimeout(() => {
            if (mountedRef.current) {
              reconnectAttemptsRef.current += 1;
              setReconnectAttempts(reconnectAttemptsRef.current);
              connect();
            }
          }, delay);
        } else if (reconnectAttemptsRef.current >= maxReconnectAttempts) {
          console.log('[useWebSocket] Max reconnect attempts reached');
        }
      };

      wsRef.current = ws;
    } catch (error) {
      setConnectionError(error);

      if (onErrorRef.current) {
        onErrorRef.current(error);
      }
    }
  }, [url, reconnectDelay, maxReconnectAttempts]);

  const disconnect = useCallback(() => {
    shouldReconnectRef.current = false;

    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }

    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }

    setIsConnected(false);
    setReconnectAttempts(0);
  }, []);

  const send = useCallback((data) => {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
      console.warn('WebSocket is not connected');
      return false;
    }

    try {
      const message = typeof data === 'string' ? data : JSON.stringify(data);
      wsRef.current.send(message);
      return true;
    } catch (error) {
      console.error('Failed to send WebSocket message:', error);
      setConnectionError(error);
      return false;
    }
  }, []);

  const reconnect = useCallback(() => {
    disconnect();
    shouldReconnectRef.current = true;
    setReconnectAttempts(0);
    reconnectAttemptsRef.current = 0;
    setTimeout(() => {
      if (mountedRef.current) {
        connect();
      }
    }, 100);
  }, [connect, disconnect]);

  useEffect(() => {
    mountedRef.current = true;

    return () => {
      mountedRef.current = false;
      shouldReconnectRef.current = false;

      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }

      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, []);

  useEffect(() => {
    console.log('[useWebSocket] Effect triggered - url:', url, 'autoConnect:', autoConnect);

    shouldReconnectRef.current = false;

    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }

    if (wsRef.current) {
      console.log('[useWebSocket] Closing existing connection');
      wsRef.current.close();
      wsRef.current = null;
    }

    setReconnectAttempts(0);
    reconnectAttemptsRef.current = 0;
    setIsConnected(false);

    if (autoConnect && url) {
      shouldReconnectRef.current = true;
      console.log('[useWebSocket] Attempting to connect to:', url);
      connect();
    }

    // Cleanup on unmount or URL/autoConnect change
    return () => {
      shouldReconnectRef.current = false;

      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
        reconnectTimeoutRef.current = null;
      }

      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }

      setIsConnected(false);
    };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [url, autoConnect]);

  return {
    isConnected,
    lastMessage,
    error: connectionError,
    reconnectAttempts,
    send,
    connect,
    disconnect,
    reconnect,
    readyState: wsRef.current?.readyState ?? WebSocket.CLOSED
  };
}
