/**
 * Audit Constants - Event types and time ranges
 */

export const EVENT_TYPES = [
  { value: '', label: 'All Events' },
  {
    value: 'oauth.token.issued',
    label: 'Token Issued',
    icon: 'key',
    color: 'bg-green-500',
  },
  {
    value: 'oauth.token.revoked',
    label: 'Token Revoked',
    icon: 'x-circle',
    color: 'bg-red-500',
  },
  {
    value: 'oauth.user.login',
    label: 'User Login',
    icon: 'user-circle',
    color: 'bg-blue-500',
  },
  {
    value: 'oauth.user.logout',
    label: 'User Logout',
    icon: 'logout',
    color: 'bg-gray-500',
  },
  {
    value: 'server.access.granted',
    label: 'Access Granted',
    icon: 'check-circle',
    color: 'bg-green-500',
  },
  {
    value: 'server.access.denied',
    label: 'Access Denied',
    icon: 'shield-exclamation',
    color: 'bg-red-500',
  },
  {
    value: 'oauth.client.created',
    label: 'Client Created',
    icon: 'plus-circle',
    color: 'bg-blue-500',
  },
  {
    value: 'oauth.client.deleted',
    label: 'Client Deleted',
    icon: 'trash',
    color: 'bg-red-500',
  },
  {
    value: 'system.config.changed',
    label: 'Config Changed',
    icon: 'cog',
    color: 'bg-yellow-500',
  },
];

export const TIME_RANGE_OPTIONS = [
  { value: '1h', label: 'Last Hour' },
  { value: '24h', label: 'Last 24 Hours' },
  { value: '7d', label: 'Last 7 Days' },
  { value: '30d', label: 'Last 30 Days' },
  { value: 'all', label: 'All Time' },
];

export const ICON_PATHS = {
  key: 'M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z',
  'x-circle': 'M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z',
  'user-circle': 'M5.121 17.804A13.937 13.937 0 0112 16c2.5 0 4.847.655 6.879 1.804M15 10a3 3 0 11-6 0 3 3 0 016 0zm6 2a9 9 0 11-18 0 9 9 0 0118 0z',
  logout: 'M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1',
  'check-circle': 'M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z',
  'shield-exclamation': 'M12 9v2m0 4h.01M5 19h14a2 2 0 002-2v-5a2 2 0 00-2-2M5 19a2 2 0 01-2-2v-5a2 2 0 012-2m0 6V9a2 2 0 012-2h10a2 2 0 012 2v10m-9 2h4a2 2 0 002-2v-5a2 2 0 00-2-2H8a2 2 0 00-2 2v5a2 2 0 002 2z',
  'plus-circle': 'M12 4v16m8-8H4',
  trash: 'M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16',
  cog: 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z',
  'document-text': 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z',
};
