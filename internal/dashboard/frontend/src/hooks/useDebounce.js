import { useState, useEffect, useRef, useCallback } from 'react';

/**
 * Custom hook for debouncing values
 *
 * @param {*} value - Value to debounce
 * @param {number} delay - Delay in milliseconds (default: 300)
 * @returns {*} Debounced value
 */
export function useDebounce(value, delay = 300) {
  const [debouncedValue, setDebouncedValue] = useState(value);

  useEffect(() => {
    const handler = setTimeout(() => {
      setDebouncedValue(value);
    }, delay);

    return () => {
      clearTimeout(handler);
    };
  }, [value, delay]);

  return debouncedValue;
}

/**
 * Custom hook for debouncing callback functions
 *
 * @param {Function} callback - Function to debounce
 * @param {number} delay - Delay in milliseconds (default: 300)
 * @param {Array} dependencies - Dependency array for the callback
 * @returns {Function} Debounced callback function
 */
export function useDebouncedCallback(callback, delay = 300, dependencies = []) {
  const timeoutRef = useRef(null);
  const callbackRef = useRef(callback);

  useEffect(() => {
    callbackRef.current = callback;
  }, [callback]);

  const debouncedCallback = useCallback((...args) => {
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
    }

    timeoutRef.current = setTimeout(() => {
      callbackRef.current(...args);
    }, delay);
  }, [delay, ...dependencies]);

  const cancel = useCallback(() => {
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
    }
  }, []);

  const flush = useCallback((...args) => {
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
    }

    callbackRef.current(...args);
  }, []);

  useEffect(() => {
    return () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
      }
    };
  }, []);

  return {
    callback: debouncedCallback,
    cancel,
    flush
  };
}

/**
 * Custom hook for throttling values
 *
 * @param {*} value - Value to throttle
 * @param {number} limit - Time limit in milliseconds (default: 300)
 * @returns {*} Throttled value
 */
export function useThrottle(value, limit = 300) {
  const [throttledValue, setThrottledValue] = useState(value);
  const lastRunRef = useRef(Date.now());

  useEffect(() => {
    const handler = setTimeout(() => {
      if (Date.now() - lastRunRef.current >= limit) {
        setThrottledValue(value);
        lastRunRef.current = Date.now();
      }
    }, limit - (Date.now() - lastRunRef.current));

    return () => {
      clearTimeout(handler);
    };
  }, [value, limit]);

  return throttledValue;
}

/**
 * Custom hook for throttling callback functions
 *
 * @param {Function} callback - Function to throttle
 * @param {number} limit - Time limit in milliseconds (default: 300)
 * @param {Array} dependencies - Dependency array for the callback
 * @returns {Function} Throttled callback function
 */
export function useThrottledCallback(callback, limit = 300, dependencies = []) {
  const lastRunRef = useRef(Date.now());
  const timeoutRef = useRef(null);
  const callbackRef = useRef(callback);

  useEffect(() => {
    callbackRef.current = callback;
  }, [callback]);

  const throttledCallback = useCallback((...args) => {
    const now = Date.now();
    const timeSinceLastRun = now - lastRunRef.current;

    if (timeSinceLastRun >= limit) {
      callbackRef.current(...args);
      lastRunRef.current = now;
    } else {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
      }

      timeoutRef.current = setTimeout(() => {
        callbackRef.current(...args);
        lastRunRef.current = Date.now();
      }, limit - timeSinceLastRun);
    }
  }, [limit, ...dependencies]);

  useEffect(() => {
    return () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
      }
    };
  }, []);

  return throttledCallback;
}
