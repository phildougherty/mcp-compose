/**
 * WebSocket Connection Manager
 * Provides WebSocket connection management with auto-reconnection logic
 */

class WebSocketManager {
  constructor(url, options = {}) {
    this.url = url;
    this.options = {
      reconnectInterval: 3000,
      maxReconnectAttempts: 10,
      reconnectBackoff: 1.5,
      heartbeatInterval: 30000,
      debug: false,
      ...options,
    };

    this.ws = null;
    this.reconnectAttempts = 0;
    this.reconnectTimer = null;
    this.heartbeatTimer = null;
    this.isIntentionallyClosed = false;
    this.listeners = {
      open: [],
      message: [],
      error: [],
      close: [],
      reconnect: [],
    };
  }

  /**
   * Connect to WebSocket server
   */
  connect() {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.log('Already connected');

      return;
    }

    this.isIntentionallyClosed = false;
    this.log('Connecting to', this.url);

    try {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsURL = this.url.startsWith('ws') ? this.url : `${protocol}//${window.location.host}${this.url}`;

      this.ws = new WebSocket(wsURL);

      this.ws.onopen = (event) => {
        this.log('Connected');
        this.reconnectAttempts = 0;
        this.startHeartbeat();
        this.emit('open', event);
      };

      this.ws.onmessage = (event) => {
        this.log('Message received:', event.data);
        this.emit('message', event);
      };

      this.ws.onerror = (event) => {
        this.log('Error:', event);
        this.emit('error', event);
      };

      this.ws.onclose = (event) => {
        this.log('Disconnected:', event.code, event.reason);
        this.stopHeartbeat();
        this.emit('close', event);

        if (!this.isIntentionallyClosed) {
          this.scheduleReconnect();
        }
      };
    } catch (error) {
      this.log('Connection error:', error);
      this.scheduleReconnect();
    }
  }

  /**
   * Disconnect from WebSocket server
   */
  disconnect() {
    this.log('Disconnecting');
    this.isIntentionallyClosed = true;
    this.stopHeartbeat();
    this.clearReconnectTimer();

    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  /**
   * Send data to WebSocket server
   * @param {any} data - Data to send (will be JSON stringified if object)
   */
  send(data) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      this.log('Cannot send: WebSocket is not connected');

      return;
    }

    const message = typeof data === 'string' ? data : JSON.stringify(data);
    this.log('Sending:', message);
    this.ws.send(message);
  }

  /**
   * Schedule reconnection attempt
   */
  scheduleReconnect() {
    if (this.reconnectAttempts >= this.options.maxReconnectAttempts) {
      this.log('Max reconnect attempts reached');

      return;
    }

    this.clearReconnectTimer();

    const delay = Math.min(
      this.options.reconnectInterval * Math.pow(this.options.reconnectBackoff, this.reconnectAttempts),
      30000
    );

    this.log(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts + 1}/${this.options.maxReconnectAttempts})`);

    this.reconnectTimer = setTimeout(() => {
      this.reconnectAttempts++;
      this.emit('reconnect', { attempt: this.reconnectAttempts });
      this.connect();
    }, delay);
  }

  /**
   * Clear reconnect timer
   */
  clearReconnectTimer() {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }

  /**
   * Start heartbeat to keep connection alive
   */
  startHeartbeat() {
    this.stopHeartbeat();

    this.heartbeatTimer = setInterval(() => {
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        this.send({ type: 'ping' });
      }
    }, this.options.heartbeatInterval);
  }

  /**
   * Stop heartbeat
   */
  stopHeartbeat() {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
  }

  /**
   * Add event listener
   * @param {string} event - Event name (open, message, error, close, reconnect)
   * @param {Function} callback - Callback function
   */
  on(event, callback) {
    if (this.listeners[event]) {
      this.listeners[event].push(callback);
    }
  }

  /**
   * Remove event listener
   * @param {string} event - Event name
   * @param {Function} callback - Callback function to remove
   */
  off(event, callback) {
    if (this.listeners[event]) {
      this.listeners[event] = this.listeners[event].filter(cb => cb !== callback);
    }
  }

  /**
   * Emit event to all listeners
   * @param {string} event - Event name
   * @param {any} data - Event data
   */
  emit(event, data) {
    if (this.listeners[event]) {
      this.listeners[event].forEach(callback => callback(data));
    }
  }

  /**
   * Get connection state
   * @returns {string} Connection state (CONNECTING, OPEN, CLOSING, CLOSED)
   */
  getState() {
    if (!this.ws) {
      return 'CLOSED';
    }

    const states = {
      [WebSocket.CONNECTING]: 'CONNECTING',
      [WebSocket.OPEN]: 'OPEN',
      [WebSocket.CLOSING]: 'CLOSING',
      [WebSocket.CLOSED]: 'CLOSED',
    };

    return states[this.ws.readyState] || 'UNKNOWN';
  }

  /**
   * Check if connected
   * @returns {boolean} True if connected
   */
  isConnected() {
    return this.ws && this.ws.readyState === WebSocket.OPEN;
  }

  /**
   * Log debug messages
   * @param {...any} args - Arguments to log
   */
  log(...args) {
    if (this.options.debug) {
      console.log('[WebSocketManager]', ...args);
    }
  }
}

/**
 * Create a WebSocket manager instance
 * @param {string} url - WebSocket URL
 * @param {Object} options - Connection options
 * @returns {WebSocketManager} WebSocket manager instance
 */
export function createWebSocketManager(url, options = {}) {
  return new WebSocketManager(url, options);
}

/**
 * Create metrics WebSocket connection
 * @returns {WebSocketManager} WebSocket manager for metrics
 */
export function createMetricsWebSocket() {
  return createWebSocketManager('/ws/metrics', {
    debug: process.env.NODE_ENV === 'development',
  });
}

/**
 * Create logs WebSocket connection
 * @returns {WebSocketManager} WebSocket manager for logs
 */
export function createLogsWebSocket() {
  return createWebSocketManager('/ws/logs', {
    debug: process.env.NODE_ENV === 'development',
  });
}

export default WebSocketManager;
export { WebSocketManager };
