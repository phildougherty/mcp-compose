import React, { useEffect, useState } from 'react';
import { useChatStore } from '../../store/chatStore';
import { chatApi } from '../../api';
import { Select } from '../shared';

export default function ModelSelector() {
  const activeSessionId = useChatStore((state) => state.activeSessionId);
  const sessions = useChatStore((state) => Array.isArray(state.sessions) ? state.sessions : []);
  const updateSession = useChatStore((state) => state.updateSession);
  const [providers, setProviders] = useState({});
  const [loading, setLoading] = useState(true);

  const activeSession = Array.isArray(sessions) ? sessions.find((s) => s.id === activeSessionId) : null;
  const selectedProvider = activeSession?.provider || 'openrouter';
  const selectedModel = activeSession?.model || 'z-ai/glm-4.6';

  useEffect(() => {
    loadProviders();
  }, []);

  const loadProviders = async () => {
    try {
      setLoading(true);
      const data = await chatApi.getChatProviders();
      setProviders(data || {});
    } catch (err) {
      console.error('Failed to load providers:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleProviderChange = async (provider) => {
    if (!activeSessionId) return;

    const models = providers[provider] || [];
    const defaultModel = models[0] || '';

    try {
      await chatApi.updateChatSession(activeSessionId, {
        provider,
        model: defaultModel,
      });

      updateSession(activeSessionId, { provider, model: defaultModel });
    } catch (err) {
      console.error('Failed to update provider:', err);
    }
  };

  const handleModelChange = async (model) => {
    if (!activeSessionId) return;

    try {
      await chatApi.updateChatSession(activeSessionId, { model });
      updateSession(activeSessionId, { model });
    } catch (err) {
      console.error('Failed to update model:', err);
    }
  };

  if (loading) {
    return (
      <div className="flex gap-2 items-center text-sm text-gray-500 dark:text-gray-400">
        <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-600" />
        Loading...
      </div>
    );
  }

  const providerOptions = Object.keys(providers).map((p) => ({
    value: p,
    label: p.charAt(0).toUpperCase() + p.slice(1),
  }));

  const modelOptions = (providers[selectedProvider] || []).map((m) => ({
    value: m,
    label: m,
  }));

  return (
    <div className="model-selector flex flex-col sm:flex-row gap-2 w-full sm:w-auto">
      <Select
        value={selectedProvider}
        onChange={handleProviderChange}
        options={providerOptions}
        placeholder="Provider"
        className="w-full sm:w-32"
        containerClassName="w-full sm:w-auto"
      />

      <Select
        value={selectedModel}
        onChange={handleModelChange}
        options={modelOptions}
        placeholder="Model"
        className="w-full sm:w-48"
        containerClassName="w-full sm:w-auto"
      />
    </div>
  );
}
