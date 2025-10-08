export interface WorkflowTemplate {
  id: string;
  name: string;
  description: string;
  category: string;
  author: string;
  downloads: number;
  rating: number;
  tags: string[];
  thumbnail?: string;
  workflow: any;
  requiredServers: string[];
  version: string;
  createdAt: string;
  updatedAt: string;
}

export interface TemplateCategory {
  id: string;
  name: string;
  icon: string;
  count: number;
}

export interface TemplateFilters {
  category: string;
  search: string;
  sortBy: 'popular' | 'recent' | 'rating';
}
