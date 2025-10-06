import { useState, useEffect, useCallback } from 'react';

/**
 * Custom hook for localStorage persistence with JSON serialization
 *
 * @param {string} key - localStorage key
 * @param {*} initialValue - Initial value if key doesn't exist
 * @param {Object} options - Configuration options
 * @param {Function} options.serializer - Custom serializer function (default: JSON.stringify)
 * @param {Function} options.deserializer - Custom deserializer function (default: JSON.parse)
 * @returns {Array} [storedValue, setValue, removeValue] tuple
 */
export function useLocalStorage(key, initialValue, options = {}) {
  const {
    serializer = JSON.stringify,
    deserializer = JSON.parse
  } = options;

  const readValue = useCallback(() => {
    if (typeof window === 'undefined') {
      return initialValue;
    }

    try {
      const item = window.localStorage.getItem(key);

      if (item === null) {
        return initialValue;
      }

      return deserializer(item);
    } catch (error) {
      console.warn(`Error reading localStorage key "${key}":`, error);
      return initialValue;
    }
  }, [key, initialValue, deserializer]);

  const [storedValue, setStoredValue] = useState(readValue);

  const setValue = useCallback((value) => {
    if (typeof window === 'undefined') {
      console.warn(`Attempted to set localStorage key "${key}" on server`);
      return;
    }

    try {
      const newValue = value instanceof Function ? value(storedValue) : value;

      window.localStorage.setItem(key, serializer(newValue));

      setStoredValue(newValue);

      window.dispatchEvent(new StorageEvent('storage', {
        key,
        newValue: serializer(newValue),
        oldValue: serializer(storedValue),
        storageArea: window.localStorage
      }));
    } catch (error) {
      console.warn(`Error setting localStorage key "${key}":`, error);
    }
  }, [key, storedValue, serializer]);

  const removeValue = useCallback(() => {
    if (typeof window === 'undefined') {
      console.warn(`Attempted to remove localStorage key "${key}" on server`);
      return;
    }

    try {
      window.localStorage.removeItem(key);

      setStoredValue(initialValue);

      window.dispatchEvent(new StorageEvent('storage', {
        key,
        newValue: null,
        oldValue: serializer(storedValue),
        storageArea: window.localStorage
      }));
    } catch (error) {
      console.warn(`Error removing localStorage key "${key}":`, error);
    }
  }, [key, initialValue, storedValue, serializer]);

  useEffect(() => {
    setStoredValue(readValue());
  }, [key]);

  useEffect(() => {
    const handleStorageChange = (e) => {
      if (e.key !== key || e.storageArea !== window.localStorage) {
        return;
      }

      try {
        if (e.newValue === null) {
          setStoredValue(initialValue);
        } else {
          setStoredValue(deserializer(e.newValue));
        }
      } catch (error) {
        console.warn(`Error handling storage change for key "${key}":`, error);
      }
    };

    if (typeof window !== 'undefined') {
      window.addEventListener('storage', handleStorageChange);

      return () => {
        window.removeEventListener('storage', handleStorageChange);
      };
    }
  }, [key, initialValue, deserializer]);

  return [storedValue, setValue, removeValue];
}

/**
 * Custom hook for sessionStorage persistence
 *
 * @param {string} key - sessionStorage key
 * @param {*} initialValue - Initial value if key doesn't exist
 * @returns {Array} [storedValue, setValue, removeValue] tuple
 */
export function useSessionStorage(key, initialValue) {
  const readValue = useCallback(() => {
    if (typeof window === 'undefined') {
      return initialValue;
    }

    try {
      const item = window.sessionStorage.getItem(key);

      if (item === null) {
        return initialValue;
      }

      return JSON.parse(item);
    } catch (error) {
      console.warn(`Error reading sessionStorage key "${key}":`, error);
      return initialValue;
    }
  }, [key, initialValue]);

  const [storedValue, setStoredValue] = useState(readValue);

  const setValue = useCallback((value) => {
    if (typeof window === 'undefined') {
      console.warn(`Attempted to set sessionStorage key "${key}" on server`);
      return;
    }

    try {
      const newValue = value instanceof Function ? value(storedValue) : value;

      window.sessionStorage.setItem(key, JSON.stringify(newValue));

      setStoredValue(newValue);
    } catch (error) {
      console.warn(`Error setting sessionStorage key "${key}":`, error);
    }
  }, [key, storedValue]);

  const removeValue = useCallback(() => {
    if (typeof window === 'undefined') {
      console.warn(`Attempted to remove sessionStorage key "${key}" on server`);
      return;
    }

    try {
      window.sessionStorage.removeItem(key);

      setStoredValue(initialValue);
    } catch (error) {
      console.warn(`Error removing sessionStorage key "${key}":`, error);
    }
  }, [key, initialValue]);

  useEffect(() => {
    setStoredValue(readValue());
  }, [key]);

  return [storedValue, setValue, removeValue];
}
