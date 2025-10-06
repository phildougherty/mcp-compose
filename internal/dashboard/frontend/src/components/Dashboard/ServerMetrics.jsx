import { formatUptime } from '../../utils/format';

const ServerMetrics = ({ metrics }) => {
  const { totalServers, runningServers, healthyServers, totalConnections, proxyUptime } = metrics;

  const metricCards = [
    {
      label: 'Total Servers',
      value: totalServers,
      subtext: 'Infrastructure',
      icon: (
        <svg className="w-6 h-6 text-white" fill="currentColor" viewBox="0 0 20 20">
          <path d="M3 4a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1V4zM3 10a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H4a1 1 0 01-1-1v-6zM14 9a1 1 0 00-1 1v6a1 1 0 001 1h2a1 1 0 001-1v-6a1 1 0 00-1-1h-2z" />
        </svg>
      ),
      bgColor: 'bg-blue-600',
      hoverBg: 'group-hover:bg-blue-500/10',
    },
    {
      label: 'Running',
      value: runningServers,
      subtext: 'Active',
      icon: (
        <svg className="w-6 h-6 text-white" fill="currentColor" viewBox="0 0 20 20">
          <path
            fillRule="evenodd"
            d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
            clipRule="evenodd"
          />
        </svg>
      ),
      bgColor: 'bg-emerald-600',
      hoverBg: 'group-hover:bg-emerald-500/10',
      pulseColor: 'bg-emerald-400',
      showPulse: true,
    },
    {
      label: 'Healthy',
      value: healthyServers,
      subtext: 'Operational',
      icon: (
        <svg className="w-6 h-6 text-white" fill="currentColor" viewBox="0 0 20 20">
          <path d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      ),
      bgColor: 'bg-green-600',
      hoverBg: 'group-hover:bg-green-500/10',
    },
    {
      label: 'Uptime',
      value: formatUptime(proxyUptime),
      subtext: 'Proxy',
      icon: (
        <svg className="w-6 h-6 text-white" fill="currentColor" viewBox="0 0 20 20">
          <path
            fillRule="evenodd"
            d="M10 18a8 8 0 100-16 8 8 0 000 16zm1-12a1 1 0 10-2 0v4a1 1 0 00.293.707l2.828 2.829a1 1 0 101.415-1.415L11 9.586V6z"
            clipRule="evenodd"
          />
        </svg>
      ),
      bgColor: 'bg-purple-600',
      hoverBg: 'group-hover:bg-purple-500/10',
      isUptime: true,
    },
    {
      label: 'Connections',
      value: totalConnections,
      subtext: 'Live',
      icon: (
        <svg className="w-6 h-6 text-white" fill="currentColor" viewBox="0 0 20 20">
          <path d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
        </svg>
      ),
      bgColor: 'bg-indigo-600',
      hoverBg: 'group-hover:bg-indigo-500/10',
    },
  ];

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5 gap-4 lg:gap-6">
      {metricCards.map((metric) => (
        <div
          key={metric.label}
          className={`relative group overflow-hidden rounded-xl bg-slate-800 border border-slate-700 p-5 transition-all hover:scale-105 hover:shadow-lg ${metric.hoverBg}`}
        >
          <div className="relative flex items-start justify-between">
            <div className="flex-1">
              <p className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">
                {metric.label}
              </p>
              <p className={`font-bold text-white mb-1 ${metric.isUptime ? 'text-2xl' : 'text-3xl'}`}>
                {metric.value}
              </p>
              <div className="flex items-center text-xs text-slate-500">
                {metric.showPulse ? (
                  <>
                    <span className={`w-1.5 h-1.5 ${metric.pulseColor} rounded-full mr-1.5 animate-pulse`} />
                    {metric.subtext}
                  </>
                ) : (
                  <>
                    <svg className="w-3 h-3 mr-1" fill="currentColor" viewBox="0 0 20 20">
                      <path d="M3 4a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1V4z" />
                    </svg>
                    {metric.subtext}
                  </>
                )}
              </div>
            </div>
            <div className="flex-shrink-0">
              <div
                className={`w-12 h-12 ${metric.bgColor} rounded-xl flex items-center justify-center shadow-md transform group-hover:rotate-6 transition-transform`}
              >
                {metric.icon}
              </div>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
};

export default ServerMetrics;
