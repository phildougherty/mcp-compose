const { createApp } = Vue;

// Create the Vue app
const app = createApp({
  components: {
    'dashboard-app': DashboardApp
  }
});

// Register components globally so they can be used within other components
app.component('log-viewer', LogViewer);
app.component('activity-viewer', ActivityViewer);
app.component('mcp-inspector', MCPInspector);

// Mount the app
app.mount('#app');