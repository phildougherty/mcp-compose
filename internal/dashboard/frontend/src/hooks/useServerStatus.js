import { useState, useEffect, useCallback, useRef } from 'react';
import { createServerStatusWebSocket } from '../api/websocket';

export function useServerStatus() {
  const [servers, setServers] = useState([]);
  const [isConnected, setIsConnected] = useState(false);
  const [error, setError] = useState(null);
  const wsRef = useRef(null);
  const reconnectTimeoutRef = useRef(null);
  const isMountedRef = useRef(true);

  const handleMessage = useCallback((event) => {
    try {
      const data = JSON.parse(event.data);

      if (data.servers) {
        const serversArray = Object.keys(data.servers || {}).map(name => ({
          name,
          ...data.servers[name]
        }));

        if (isMountedRef.current) {
          setServers(serversArray);
          setError(null);
        }
      }
    } catch (err) {
      console.error('Failed to parse server status message:', err);
      if (isMountedRef.current) {
        setError('Failed to parse server status data');
      }
    }
  }, []);

  const handleOpen = useCallback(() => {
    console.log('Server status WebSocket connected');
    if (isMountedRef.current) {
      setIsConnected(true);
      setError(null);
    }
  }, []);

  const handleError = useCallback((event) => {
    console.error('Server status WebSocket error:', event);
    if (isMountedRef.current) {
      setError('WebSocket connection error');
    }
  }, []);

  const handleClose = useCallback(() => {
    console.log('Server status WebSocket closed');
    if (isMountedRef.current) {
      setIsConnected(false);
    }
  }, []);

  const connect = useCallback(() => {
    if (wsRef.current && wsRef.current.isConnected()) {
      return;
    }

    try {
      if (wsRef.current) {
        wsRef.current.disconnect();
      }

      wsRef.current = createServerStatusWebSocket();

      wsRef.current.on('open', handleOpen);
      wsRef.current.on('message', handleMessage);
      wsRef.current.on('error', handleError);
      wsRef.current.on('close', handleClose);

      wsRef.current.connect();
    } catch (err) {
      console.error('Failed to create WebSocket connection:', err);
      if (isMountedRef.current) {
        setError('Failed to establish WebSocket connection');
      }
    }
  }, [handleOpen, handleMessage, handleError, handleClose]);

  const disconnect = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }

    if (wsRef.current) {
      wsRef.current.disconnect();
      wsRef.current = null;
    }
  }, []);

  useEffect(() => {
    isMountedRef.current = true;

    const fallbackFetch = async () => {
      try {
        const response = await fetch('/api/servers');
        if (response.ok) {
          const data = await response.json();
          const serversArray = Object.keys(data || {}).map(name => ({
            name,
            ...data[name]
          }));
          if (isMountedRef.current) {
            setServers(serversArray);
          }
        }
      } catch (err) {
        console.error('Fallback fetch failed:', err);
      }
    };

    fallbackFetch();

    connect();

    return () => {
      isMountedRef.current = false;
      disconnect();
    };
  }, [connect, disconnect]);

  return {
    servers,
    isConnected,
    error,
    reconnect: connect
  };
}
