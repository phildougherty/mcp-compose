/**
 * ExportButton Component - CSV export button
 */

import React, { useState } from 'react';
import useAuditStore from '../../store/auditStore';
import { downloadAuditLogs } from '../../api/audit';
import { useToast } from '../../hooks/useToast';
import { Button } from '../shared';

function ExportButton() {
  const { filters } = useAuditStore();
  const { success, error: showError } = useToast();
  const [exporting, setExporting] = useState(false);

  const handleExport = async () => {
    setExporting(true);

    try {
      const params = {
        ...(filters.event && { event: filters.event }),
        ...(filters.success !== '' && { success: filters.success }),
        ...(filters.timeRange !== 'all' && { timeRange: filters.timeRange }),
        ...(filters.search && { search: filters.search }),
        format: 'csv',
        limit: 10000,
      };

      const filename = `audit-log-${new Date().toISOString().split('T')[0]}.csv`;
      await downloadAuditLogs(params, filename);

      success('Audit log exported successfully');
    } catch (err) {
      console.error('Export failed:', err);
      showError(`Export failed: ${err.message}`);
    } finally {
      setExporting(false);
    }
  };

  return (
    <Button
      onClick={handleExport}
      disabled={exporting}
      loading={exporting}
      variant="success"
      size="md"
    >
      <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth={2}
          d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
        />
      </svg>
      {exporting ? 'Exporting...' : 'Export CSV'}
    </Button>
  );
}

export default ExportButton;
