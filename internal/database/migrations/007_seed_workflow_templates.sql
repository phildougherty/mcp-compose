-- Seed workflow templates from frontend mock data

-- Template 1: GitHub Issue Tracker
INSERT INTO templates.workflow_templates
(id, name, description, category, author, tags, version, downloads, rating, workflow_definition, required_servers, difficulty, created_at, updated_at)
VALUES
('template-1', 'GitHub Issue Tracker',
'Automatically track GitHub issues and sync with your database. Creates notifications for new issues and updates.',
'Data Engineering', 'MCP Team',
ARRAY['github', 'database', 'automation'],
'1.2.0', 1250, 4.8,
'{"nodes": [], "edges": []}',
ARRAY['mcp-server-github', 'mcp-server-postgres'],
'intermediate',
'2025-09-15T10:00:00Z', '2025-10-01T14:30:00Z');

-- Template 2: Slack Alert System
INSERT INTO templates.workflow_templates
(id, name, description, category, author, tags, version, downloads, rating, workflow_definition, required_servers, difficulty, created_at, updated_at)
VALUES
('template-2', 'Slack Alert System',
'Monitor server health and send alerts to Slack channels when issues are detected. Includes escalation policies.',
'Monitoring & Alerts', 'DevOps Pro',
ARRAY['slack', 'monitoring', 'alerts'],
'2.0.1', 890, 4.6,
'{"nodes": [], "edges": []}',
ARRAY['mcp-server-slack', 'mcp-server-prometheus'],
'intermediate',
'2025-08-20T08:15:00Z', '2025-09-28T16:45:00Z');

-- Template 3: AI Content Generator
INSERT INTO templates.workflow_templates
(id, name, description, category, author, tags, version, downloads, rating, workflow_definition, required_servers, difficulty, created_at, updated_at)
VALUES
('template-3', 'AI Content Generator',
'Generate blog posts, social media content, and marketing copy using AI. Supports multiple formats and tone customization.',
'Content Generation', 'Content AI Labs',
ARRAY['ai', 'content', 'marketing', 'openai'],
'1.5.0', 2340, 4.9,
'{"nodes": [], "edges": []}',
ARRAY['mcp-server-openai', 'mcp-server-filesystem'],
'beginner',
'2025-07-10T12:00:00Z', '2025-10-05T09:20:00Z');

-- Template 4: Customer Support Automation
INSERT INTO templates.workflow_templates
(id, name, description, category, author, tags, version, downloads, rating, workflow_definition, required_servers, difficulty, created_at, updated_at)
VALUES
('template-4', 'Customer Support Automation',
'Automate customer support ticket routing and responses. Integrates with Zendesk, email, and knowledge base.',
'Customer Support', 'Support Team',
ARRAY['support', 'automation', 'zendesk'],
'1.1.0', 675, 4.5,
'{"nodes": [], "edges": []}',
ARRAY['mcp-server-zendesk', 'mcp-server-email'],
'intermediate',
'2025-09-01T11:30:00Z', '2025-09-25T13:15:00Z');

-- Template 5: Email Campaign Manager
INSERT INTO templates.workflow_templates
(id, name, description, category, author, tags, version, downloads, rating, workflow_definition, required_servers, difficulty, created_at, updated_at)
VALUES
('template-5', 'Email Campaign Manager',
'Schedule and send email campaigns with personalization. Track opens, clicks, and conversions automatically.',
'Marketing Automation', 'Marketing Genius',
ARRAY['email', 'marketing', 'campaigns'],
'2.1.0', 1560, 4.7,
'{"nodes": [], "edges": []}',
ARRAY['mcp-server-sendgrid', 'mcp-server-analytics'],
'intermediate',
'2025-06-15T09:00:00Z', '2025-10-02T10:30:00Z');

-- Template 6: CI/CD Pipeline Monitor
INSERT INTO templates.workflow_templates
(id, name, description, category, author, tags, version, downloads, rating, workflow_definition, required_servers, difficulty, created_at, updated_at)
VALUES
('template-6', 'CI/CD Pipeline Monitor',
'Monitor CI/CD pipelines across multiple platforms. Get notified of build failures and deployment issues.',
'DevOps', 'DevOps Ninja',
ARRAY['cicd', 'devops', 'monitoring'],
'1.3.0', 980, 4.8,
'{"nodes": [], "edges": []}',
ARRAY['mcp-server-github', 'mcp-server-gitlab', 'mcp-server-slack'],
'advanced',
'2025-08-05T14:20:00Z', '2025-09-30T15:45:00Z');

-- Template 7: Data Backup & Sync
INSERT INTO templates.workflow_templates
(id, name, description, category, author, tags, version, downloads, rating, workflow_definition, required_servers, difficulty, created_at, updated_at)
VALUES
('template-7', 'Data Backup & Sync',
'Automated data backup and synchronization across multiple storage providers. Includes encryption and versioning.',
'Data Engineering', 'Data Guardian',
ARRAY['backup', 'sync', 'storage'],
'1.0.5', 1120, 4.6,
'{"nodes": [], "edges": []}',
ARRAY['mcp-server-s3', 'mcp-server-filesystem'],
'beginner',
'2025-09-10T16:00:00Z', '2025-09-29T11:20:00Z');

-- Template 8: Social Media Scheduler
INSERT INTO templates.workflow_templates
(id, name, description, category, author, tags, version, downloads, rating, workflow_definition, required_servers, difficulty, created_at, updated_at)
VALUES
('template-8', 'Social Media Scheduler',
'Schedule and publish content across multiple social media platforms. Includes analytics and engagement tracking.',
'Marketing Automation', 'Social Pro',
ARRAY['social-media', 'scheduling', 'marketing'],
'2.2.0', 1890, 4.9,
'{"nodes": [], "edges": []}',
ARRAY['mcp-server-twitter', 'mcp-server-facebook', 'mcp-server-instagram'],
'intermediate',
'2025-07-20T10:30:00Z', '2025-10-04T14:00:00Z');

-- Template 9: Log Aggregation System
INSERT INTO templates.workflow_templates
(id, name, description, category, author, tags, version, downloads, rating, workflow_definition, required_servers, difficulty, created_at, updated_at)
VALUES
('template-9', 'Log Aggregation System',
'Collect, parse, and analyze logs from multiple sources. Create alerts based on log patterns and anomalies.',
'Monitoring & Alerts', 'LogMaster',
ARRAY['logs', 'monitoring', 'analytics'],
'1.4.0', 745, 4.4,
'{"nodes": [], "edges": []}',
ARRAY['mcp-server-elasticsearch', 'mcp-server-filesystem'],
'advanced',
'2025-08-12T13:45:00Z', '2025-09-27T09:15:00Z');

-- Template 10: Document Generator
INSERT INTO templates.workflow_templates
(id, name, description, category, author, tags, version, downloads, rating, workflow_definition, required_servers, difficulty, created_at, updated_at)
VALUES
('template-10', 'Document Generator',
'Generate PDF documents, reports, and invoices from templates. Supports dynamic data and customizable layouts.',
'Content Generation', 'DocGen Inc',
ARRAY['pdf', 'documents', 'reports'],
'1.6.0', 1340, 4.7,
'{"nodes": [], "edges": []}',
ARRAY['mcp-server-pdf', 'mcp-server-filesystem'],
'beginner',
'2025-07-05T15:20:00Z', '2025-10-03T12:40:00Z');

-- Template 11: Database ETL Pipeline
INSERT INTO templates.workflow_templates
(id, name, description, category, author, tags, version, downloads, rating, workflow_definition, required_servers, difficulty, created_at, updated_at)
VALUES
('template-11', 'Database ETL Pipeline',
'Extract, transform, and load data between different databases. Supports scheduling and incremental updates.',
'Data Engineering', 'ETL Expert',
ARRAY['etl', 'database', 'migration'],
'1.1.0', 890, 4.5,
'{"nodes": [], "edges": []}',
ARRAY['mcp-server-postgres', 'mcp-server-mysql'],
'advanced',
'2025-08-25T11:00:00Z', '2025-09-26T14:30:00Z');

-- Template 12: Infrastructure Cost Monitor
INSERT INTO templates.workflow_templates
(id, name, description, category, author, tags, version, downloads, rating, workflow_definition, required_servers, difficulty, created_at, updated_at)
VALUES
('template-12', 'Infrastructure Cost Monitor',
'Track cloud infrastructure costs and send budget alerts. Provides detailed cost breakdowns and recommendations.',
'DevOps', 'CloudSaver',
ARRAY['aws', 'cost', 'monitoring'],
'2.0.0', 1450, 4.8,
'{"nodes": [], "edges": []}',
ARRAY['mcp-server-aws', 'mcp-server-slack'],
'intermediate',
'2025-06-30T09:30:00Z', '2025-10-01T16:20:00Z');

-- Template 13: Customer Feedback Analyzer
INSERT INTO templates.workflow_templates
(id, name, description, category, author, tags, version, downloads, rating, workflow_definition, required_servers, difficulty, created_at, updated_at)
VALUES
('template-13', 'Customer Feedback Analyzer',
'Collect and analyze customer feedback using AI. Generates sentiment analysis and actionable insights.',
'Customer Support', 'Feedback AI',
ARRAY['feedback', 'ai', 'sentiment'],
'1.2.0', 620, 4.6,
'{"nodes": [], "edges": []}',
ARRAY['mcp-server-openai', 'mcp-server-postgres'],
'intermediate',
'2025-09-05T12:15:00Z', '2025-09-28T10:45:00Z');

-- Template 14: Security Vulnerability Scanner
INSERT INTO templates.workflow_templates
(id, name, description, category, author, tags, version, downloads, rating, workflow_definition, required_servers, difficulty, created_at, updated_at)
VALUES
('template-14', 'Security Vulnerability Scanner',
'Scan code repositories for security vulnerabilities. Integrates with security databases and creates tickets.',
'DevOps', 'SecOps Team',
ARRAY['security', 'scanning', 'vulnerabilities'],
'1.7.0', 1150, 4.9,
'{"nodes": [], "edges": []}',
ARRAY['mcp-server-github', 'mcp-server-snyk'],
'advanced',
'2025-07-15T14:00:00Z', '2025-10-06T11:30:00Z');

-- Template 15: Inventory Management System
INSERT INTO templates.workflow_templates
(id, name, description, category, author, tags, version, downloads, rating, workflow_definition, required_servers, difficulty, created_at, updated_at)
VALUES
('template-15', 'Inventory Management System',
'Track inventory levels and automate reordering. Sends alerts for low stock and generates purchase orders.',
'Data Engineering', 'Inventory Pro',
ARRAY['inventory', 'automation', 'erp'],
'1.0.0', 530, 4.4,
'{"nodes": [], "edges": []}',
ARRAY['mcp-server-postgres', 'mcp-server-email'],
'beginner',
'2025-09-12T10:00:00Z', '2025-09-24T15:00:00Z');
