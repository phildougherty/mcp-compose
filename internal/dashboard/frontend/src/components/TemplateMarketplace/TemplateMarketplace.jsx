import React, { useState, useMemo } from 'react';
import {
  MagnifyingGlassIcon,
  FunnelIcon,
  Squares2X2Icon,
  CircleStackIcon,
  BellAlertIcon,
  DocumentTextIcon,
  ChatBubbleLeftRightIcon,
  CodeBracketIcon,
  SparklesIcon,
  ArrowsUpDownIcon,
} from '@heroicons/react/24/outline';
import { Button, Badge, EmptyState, Modal } from '../shared';
import TemplateCard from './TemplateCard';
import { mockTemplates, templateCategories } from './mockTemplates';

const categoryIcons = {
  'Squares2X2Icon': Squares2X2Icon,
  'CircleStackIcon': CircleStackIcon,
  'BellAlertIcon': BellAlertIcon,
  'DocumentTextIcon': DocumentTextIcon,
  'ChatBubbleLeftRightIcon': ChatBubbleLeftRightIcon,
  'MegaphoneIcon': SparklesIcon,
  'CodeBracketIcon': CodeBracketIcon,
};

const TemplateMarketplace = () => {
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedCategory, setSelectedCategory] = useState('all');
  const [sortBy, setSortBy] = useState('popular');
  const [showFilters, setShowFilters] = useState(false);
  const [showMissingServersModal, setShowMissingServersModal] = useState(false);
  const [missingServers, setMissingServers] = useState([]);
  const [selectedTemplate, setSelectedTemplate] = useState(null);

  const sortedAndFilteredTemplates = useMemo(() => {
    let filtered = mockTemplates;

    if (selectedCategory !== 'all') {
      const categoryName = templateCategories.find(c => c.id === selectedCategory)?.name;
      filtered = filtered.filter(t => t.category === categoryName);
    }

    if (searchQuery) {
      const query = searchQuery.toLowerCase();
      filtered = filtered.filter(t =>
        t.name.toLowerCase().includes(query) ||
        t.description.toLowerCase().includes(query) ||
        t.tags.some(tag => tag.toLowerCase().includes(query)) ||
        t.author.toLowerCase().includes(query)
      );
    }

    const sorted = [...filtered].sort((a, b) => {
      switch (sortBy) {
        case 'popular':
          return b.downloads - a.downloads;
        case 'recent':
          return new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime();
        case 'rating':
          return b.rating - a.rating;
        default:
          return 0;
      }
    });

    return sorted;
  }, [selectedCategory, searchQuery, sortBy]);

  const handleInstallTemplate = async (template) => {
    setSelectedTemplate(template);

    const missing = template.requiredServers.filter(server => {
      return Math.random() > 0.7;
    });

    if (missing.length > 0) {
      setMissingServers(missing);
      setShowMissingServersModal(true);
      return;
    }

    alert(`Template "${template.name}" installed successfully! Redirecting to workflow builder...`);
  };

  const handleProceedWithInstall = () => {
    setShowMissingServersModal(false);
    alert(`Installing required servers and template "${selectedTemplate.name}"...`);
  };

  const handleCancelInstall = () => {
    setShowMissingServersModal(false);
    setSelectedTemplate(null);
    setMissingServers([]);
  };

  const getCategoryIcon = (iconName) => {
    const IconComponent = categoryIcons[iconName];
    return IconComponent || Squares2X2Icon;
  };

  return (
    <div className="w-full max-w-full overflow-x-hidden">
      <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-4 lg:p-6 mb-6">
        <div className="flex flex-col space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-3">
              <div className="flex-shrink-0">
                <div className="w-10 h-10 bg-gradient-to-br from-purple-500 to-pink-600 rounded-lg flex items-center justify-center shadow-lg">
                  <SparklesIcon className="w-6 h-6 text-white" />
                </div>
              </div>
              <div>
                <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
                  Workflow Templates
                </h3>
                <p className="text-sm text-gray-500 dark:text-gray-400">
                  Browse and install pre-built workflow templates
                </p>
              </div>
            </div>
            <Badge variant="primary" size="md">
              {mockTemplates.length} templates
            </Badge>
          </div>

          <div className="flex flex-col sm:flex-row gap-3">
            <div className="flex-1 relative">
              <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
                <MagnifyingGlassIcon className="h-5 w-5 text-gray-400" aria-hidden="true" />
              </div>
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="Search templates by name, description, or tags..."
                className="block w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 pl-10 pr-3 py-2.5 text-sm text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
              />
            </div>

            <div className="flex gap-2">
              <Button
                onClick={() => setShowFilters(!showFilters)}
                variant={showFilters ? 'primary' : 'secondary'}
                size="md"
                className="gap-2"
              >
                <FunnelIcon className="h-5 w-5" />
                Filters
                {(selectedCategory !== 'all' || sortBy !== 'popular') && (
                  <Badge variant="primary" size="sm">
                    {(selectedCategory !== 'all' ? 1 : 0) + (sortBy !== 'popular' ? 1 : 0)}
                  </Badge>
                )}
              </Button>

              <select
                value={sortBy}
                onChange={(e) => setSortBy(e.target.value)}
                className="rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 px-3 py-2 text-sm text-gray-900 dark:text-white focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
              >
                <option value="popular">Most Popular</option>
                <option value="recent">Recently Updated</option>
                <option value="rating">Highest Rated</option>
              </select>
            </div>
          </div>

          {showFilters && (
            <div className="pt-4 border-t border-gray-200 dark:border-gray-700">
              <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
                Categories
              </h4>
              <div className="flex flex-wrap gap-2">
                {templateCategories.map((category) => {
                  const IconComponent = getCategoryIcon(category.icon);
                  return (
                    <button
                      key={category.id}
                      onClick={() => setSelectedCategory(category.id)}
                      className={`inline-flex items-center gap-2 px-3 py-2 rounded-lg text-sm font-medium transition-colors ${
                        selectedCategory === category.id
                          ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/20 dark:text-blue-400 border border-blue-200 dark:border-blue-800'
                          : 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300 border border-gray-200 dark:border-gray-600 hover:bg-gray-200 dark:hover:bg-gray-600'
                      }`}
                    >
                      <IconComponent className="h-4 w-4" />
                      {category.name}
                      <Badge
                        variant={selectedCategory === category.id ? 'primary' : 'default'}
                        size="sm"
                      >
                        {category.count}
                      </Badge>
                    </button>
                  );
                })}
              </div>
            </div>
          )}
        </div>
      </div>

      <main className="px-3 sm:px-4 lg:px-6 py-4 max-w-full overflow-x-hidden w-full">
        {sortedAndFilteredTemplates.length === 0 ? (
          <EmptyState
            icon={
              <MagnifyingGlassIcon className="w-12 h-12" />
            }
            title="No templates found"
            description="Try adjusting your search or filter criteria to find what you're looking for."
          />
        ) : (
          <>
            <div className="mb-4 flex items-center justify-between">
              <p className="text-sm text-gray-600 dark:text-gray-400">
                Showing {sortedAndFilteredTemplates.length} template{sortedAndFilteredTemplates.length !== 1 ? 's' : ''}
              </p>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
              {sortedAndFilteredTemplates.map((template) => (
                <TemplateCard
                  key={template.id}
                  template={template}
                  onInstall={handleInstallTemplate}
                />
              ))}
            </div>
          </>
        )}
      </main>

      {showMissingServersModal && (
        <Modal
          isOpen={showMissingServersModal}
          onClose={handleCancelInstall}
          title="Missing Required Servers"
          size="md"
        >
          <div className="p-6">
            <p className="text-sm text-gray-600 dark:text-gray-300 mb-4">
              This template requires the following servers that are not currently installed:
            </p>
            <ul className="space-y-2 mb-6">
              {missingServers.map((server, index) => (
                <li
                  key={index}
                  className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300 bg-gray-50 dark:bg-gray-700 rounded-lg p-3"
                >
                  <div className="w-2 h-2 bg-orange-500 rounded-full"></div>
                  <span className="font-medium">{server}</span>
                </li>
              ))}
            </ul>
            <p className="text-sm text-gray-600 dark:text-gray-300 mb-6">
              Would you like to install these servers along with the template?
            </p>
            <div className="flex justify-end gap-3">
              <Button onClick={handleCancelInstall} variant="secondary" size="md">
                Cancel
              </Button>
              <Button onClick={handleProceedWithInstall} variant="primary" size="md">
                Install All
              </Button>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
};

export default TemplateMarketplace;
