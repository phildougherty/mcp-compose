import { useDashboardStore } from '../../store/dashboardStore';
import { SearchInput, Select } from '../shared';

const ServerFilters = () => {
  const searchQuery = useDashboardStore((state) => state.searchQuery);
  const statusFilter = useDashboardStore((state) => state.statusFilter);
  const sortBy = useDashboardStore((state) => state.sortBy);
  const setSearchQuery = useDashboardStore((state) => state.setSearchQuery);
  const setStatusFilter = useDashboardStore((state) => state.setStatusFilter);
  const setSorting = useDashboardStore((state) => state.setSorting);
  const metrics = useDashboardStore((state) => state.metrics);

  const statusOptions = [
    { value: 'all', label: `All Servers (${metrics.totalServers})` },
    { value: 'running', label: `Running (${metrics.runningServers})` },
    { value: 'stopped', label: `Stopped (${metrics.totalServers - metrics.runningServers})` },
    { value: 'healthy', label: `Healthy (${metrics.healthyServers})` },
  ];

  const sortOptions = [
    { value: 'name', label: 'Sort: Name' },
    { value: 'status', label: 'Sort: Status' },
    { value: 'health', label: 'Sort: Health' },
    { value: 'tools', label: 'Sort: Tools' },
  ];

  return (
    <div className="rounded-xl bg-slate-800 border border-slate-700 p-6 shadow-md">
      <div className="flex flex-col space-y-4 lg:flex-row lg:items-center lg:justify-between lg:space-y-0 lg:space-x-6">
        <div className="flex-1 max-w-2xl">
          <SearchInput
            value={searchQuery}
            onChange={setSearchQuery}
            placeholder="Search servers by name..."
            className="w-full"
          />
        </div>

        <div className="flex flex-col sm:flex-row space-y-3 sm:space-y-0 sm:space-x-3">
          <Select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            options={statusOptions}
            className="min-w-[200px]"
          />

          <Select
            value={sortBy}
            onChange={(e) => setSorting(e.target.value)}
            options={sortOptions}
            className="min-w-[180px]"
          />
        </div>
      </div>
    </div>
  );
};

export default ServerFilters;
