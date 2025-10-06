/**
 * OAuth Client Form Component
 * Form for creating/editing OAuth clients
 */

import { useState } from 'react';
import { Modal, Input, Button, Checkbox } from '../shared';
import { registerOAuthClient } from '../../api/oauth';
import { useToast } from '../../hooks';

export default function ClientForm({ onClose, onClientCreated }) {
  const [formData, setFormData] = useState({
    name: '',
    description: '',
    redirect_uris: `${window.location.origin}/oauth/callback`,
    public: true,
  });
  const [creating, setCreating] = useState(false);
  const { success, error: showError } = useToast();

  const handleSubmit = async (e) => {
    e.preventDefault();
    setCreating(true);

    try {
      const clientData = {
        client_name: formData.name,
        client_description: formData.description,
        redirect_uris: formData.redirect_uris
          .split('\n')
          .filter((uri) => uri.trim()),
        grant_types: formData.public
          ? ['authorization_code', 'refresh_token']
          : ['authorization_code', 'client_credentials', 'refresh_token'],
        response_types: ['code'],
        token_endpoint_auth_method: formData.public ? 'none' : 'client_secret_post',
      };

      const client = await registerOAuthClient(clientData);
      success('OAuth client created successfully');
      onClientCreated?.(client);
    } catch (err) {
      showError(`Failed to create client: ${err.message}`);
    } finally{
      setCreating(false);
    }
  };

  return (
    <Modal
      isOpen={true}
      onClose={onClose}
      title="Register New OAuth Client"
      size="lg"
    >
      <form onSubmit={handleSubmit} className="space-y-5">
        <Input
          label="Client Name"
          type="text"
          value={formData.name}
          onChange={(e) => setFormData({ ...formData, name: e.target.value })}
          required
          placeholder="My Application"
        />

        <Input
          label="Description"
          type="text"
          value={formData.description}
          onChange={(e) =>
            setFormData({ ...formData, description: e.target.value })
          }
          placeholder="Brief description of your application"
        />

        <div>
          <label className="block text-sm font-bold text-slate-300 mb-2">
            Redirect URIs *
          </label>
          <textarea
            value={formData.redirect_uris}
            onChange={(e) =>
              setFormData({ ...formData, redirect_uris: e.target.value })
            }
            rows={3}
            required
            className="w-full rounded-xl border-2 border-slate-600 bg-slate-800 text-white px-4 py-3 focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20 transition-colors duration-200"
            placeholder="https://yourapp.com/oauth/callback&#10;http://localhost:3000/callback"
          />
          <p className="text-xs text-slate-400 mt-2">One URI per line</p>
        </div>

        <Checkbox
          checked={formData.public}
          onChange={(checked) => setFormData({ ...formData, public: checked })}
          label="Public Client (mobile apps, SPAs - no client secret)"
        />

        <div className="flex flex-col sm:flex-row justify-end gap-3 pt-6 border-t-2 border-slate-700">
          <Button
            type="button"
            onClick={onClose}
            variant="secondary"
            className="w-full sm:w-auto"
          >
            Cancel
          </Button>
          <Button
            type="submit"
            disabled={creating || !formData.name.trim()}
            className="w-full sm:w-auto bg-gradient-to-r from-green-600 to-emerald-600 hover:from-green-700 hover:to-emerald-700 shadow-lg shadow-green-600/30"
          >
            {creating ? (
              <>
                <svg
                  className="animate-spin -ml-1 mr-2 h-5 w-5 text-white"
                  fill="none"
                  viewBox="0 0 24 24"
                >
                  <circle
                    className="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    strokeWidth="4"
                  />
                  <path
                    className="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  />
                </svg>
                Creating...
              </>
            ) : (
              'Create Client'
            )}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
