const { createApp } = Vue;

// Create the Vue app
const app = createApp({
  components: {
    'dashboard-app': DashboardApp
  }
});

// Register components globally so they can be used within other components
app.component('server-manager', ServerManager);
app.component('log-viewer', LogViewer);
app.component('metrics-display', MetricsDisplay);
app.component('activity-viewer', ActivityViewer);

// Mount the app
app.mount('#app');