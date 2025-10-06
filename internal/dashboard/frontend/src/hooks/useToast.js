import { useState, useCallback, useRef } from 'react';

let toastIdCounter = 0;

/**
 * Toast notification object structure
 * @typedef {Object} Toast
 * @property {number} id - Unique toast identifier
 * @property {string} message - Toast message text
 * @property {string} type - Toast type ('success', 'error', 'warning', 'info')
 * @property {number} duration - Display duration in ms
 * @property {number} timestamp - Creation timestamp
 */

/**
 * Custom hook for toast notification management
 *
 * @param {Object} options - Configuration options
 * @param {number} options.defaultDuration - Default toast duration in ms (default: 3000)
 * @param {number} options.maxToasts - Maximum number of toasts to display (default: 5)
 * @returns {Object} Toast state and control functions
 */
export function useToast(options = {}) {
  const { defaultDuration = 3000, maxToasts = 5 } = options;

  const [toasts, setToasts] = useState([]);
  const timeoutsRef = useRef(new Map());

  const removeToast = useCallback((id) => {
    setToasts((prev) => prev.filter((toast) => toast.id !== id));

    const timeout = timeoutsRef.current.get(id);
    if (timeout) {
      clearTimeout(timeout);
      timeoutsRef.current.delete(id);
    }
  }, []);

  const addToast = useCallback((message, type = 'info', duration = defaultDuration) => {
    const id = ++toastIdCounter;

    const toast = {
      id,
      message,
      type,
      duration,
      timestamp: Date.now()
    };

    setToasts((prev) => {
      const newToasts = [toast, ...prev];
      return newToasts.slice(0, maxToasts);
    });

    if (duration > 0) {
      const timeout = setTimeout(() => {
        removeToast(id);
      }, duration);

      timeoutsRef.current.set(id, timeout);
    }

    return id;
  }, [defaultDuration, maxToasts, removeToast]);

  const success = useCallback((message, duration) => {
    return addToast(message, 'success', duration);
  }, [addToast]);

  const error = useCallback((message, duration) => {
    return addToast(message, 'error', duration);
  }, [addToast]);

  const warning = useCallback((message, duration) => {
    return addToast(message, 'warning', duration);
  }, [addToast]);

  const info = useCallback((message, duration) => {
    return addToast(message, 'info', duration);
  }, [addToast]);

  const clearAll = useCallback(() => {
    timeoutsRef.current.forEach((timeout) => clearTimeout(timeout));
    timeoutsRef.current.clear();
    setToasts([]);
  }, []);

  const clearToast = useCallback((id) => {
    removeToast(id);
  }, [removeToast]);

  return {
    toasts,
    addToast,
    success,
    error,
    warning,
    info,
    removeToast: clearToast,
    clearAll
  };
}
