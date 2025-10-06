import { useState, useEffect, useCallback, useRef } from 'react';

/**
 * Custom hook for API data fetching with loading and error states
 *
 * @param {Function} apiFunction - API function to call (should return a Promise)
 * @param {Object} options - Configuration options
 * @param {boolean} options.immediate - Execute immediately on mount (default: true)
 * @param {Array} options.dependencies - Dependencies array for refetching (default: [])
 * @param {Function} options.onSuccess - Success callback
 * @param {Function} options.onError - Error callback
 * @param {*} options.initialData - Initial data value (default: null)
 * @returns {Object} API state and control functions
 */
export function useApi(apiFunction, options = {}) {
  const {
    immediate = true,
    dependencies = [],
    onSuccess,
    onError,
    initialData = null
  } = options;

  const [data, setData] = useState(initialData);
  const [loading, setLoading] = useState(immediate);
  const [error, setError] = useState(null);
  const [isSuccess, setIsSuccess] = useState(false);

  const mountedRef = useRef(true);
  const abortControllerRef = useRef(null);

  const execute = useCallback(async (...args) => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
    }

    abortControllerRef.current = new AbortController();

    setLoading(true);
    setError(null);
    setIsSuccess(false);

    try {
      const result = await apiFunction(...args, {
        signal: abortControllerRef.current.signal
      });

      if (!mountedRef.current) return;

      setData(result);
      setIsSuccess(true);
      setError(null);

      if (onSuccess) {
        onSuccess(result);
      }

      return result;
    } catch (err) {
      if (!mountedRef.current) return;

      if (err.name === 'AbortError') {
        return;
      }

      setError(err);
      setIsSuccess(false);

      if (onError) {
        onError(err);
      }

      throw err;
    } finally {
      if (mountedRef.current) {
        setLoading(false);
      }
    }
  }, [apiFunction, onSuccess, onError]);

  const reset = useCallback(() => {
    setData(initialData);
    setLoading(false);
    setError(null);
    setIsSuccess(false);
  }, [initialData]);

  const refetch = useCallback(() => {
    return execute();
  }, [execute]);

  useEffect(() => {
    mountedRef.current = true;

    if (immediate) {
      execute();
    }

    return () => {
      mountedRef.current = false;

      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }
    };
  }, [immediate, execute, ...dependencies]);

  return {
    data,
    loading,
    error,
    isSuccess,
    execute,
    refetch,
    reset
  };
}

/**
 * Custom hook for API mutations (POST, PUT, DELETE)
 *
 * @param {Function} mutationFunction - Mutation function to call (should return a Promise)
 * @param {Object} options - Configuration options
 * @param {Function} options.onSuccess - Success callback
 * @param {Function} options.onError - Error callback
 * @returns {Object} Mutation state and control functions
 */
export function useMutation(mutationFunction, options = {}) {
  const { onSuccess, onError } = options;

  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [isSuccess, setIsSuccess] = useState(false);

  const mountedRef = useRef(true);

  const mutate = useCallback(async (...args) => {
    setLoading(true);
    setError(null);
    setIsSuccess(false);

    try {
      const result = await mutationFunction(...args);

      if (!mountedRef.current) return;

      setData(result);
      setIsSuccess(true);
      setError(null);

      if (onSuccess) {
        onSuccess(result);
      }

      return result;
    } catch (err) {
      if (!mountedRef.current) return;

      setError(err);
      setIsSuccess(false);

      if (onError) {
        onError(err);
      }

      throw err;
    } finally {
      if (mountedRef.current) {
        setLoading(false);
      }
    }
  }, [mutationFunction, onSuccess, onError]);

  const reset = useCallback(() => {
    setData(null);
    setLoading(false);
    setError(null);
    setIsSuccess(false);
  }, []);

  useEffect(() => {
    mountedRef.current = true;

    return () => {
      mountedRef.current = false;
    };
  }, []);

  return {
    data,
    loading,
    error,
    isSuccess,
    mutate,
    reset
  };
}
