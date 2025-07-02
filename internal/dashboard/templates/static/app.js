// app.js
// Check if Vue app already exists
if (typeof window.mcpApp === 'undefined') {
  const { createApp } = Vue;
  
  // Create the Vue app
  window.mcpApp = createApp({
      components: {
          'dashboard-app': DashboardApp
      }
  });
  
  // Register all components globally
  window.mcpApp.component('task-scheduler', TaskScheduler);
  window.mcpApp.component('log-viewer', LogViewer);
  window.mcpApp.component('memory-viewer', MemoryViewer);
  window.mcpApp.component('activity-viewer', ActivityViewer);
  window.mcpApp.component('mcp-inspector', MCPInspector);
  window.mcpApp.component('oauth-config', OAuthConfig);
  window.mcpApp.component('audit-log', AuditLog);
  window.mcpApp.component('server-oauth-config', ServerOAuthConfig);
  
  // Mount the app
  window.mcpApp.mount('#app');
}