import React from 'react';
import { FixedSizeList as List } from 'react-window';
import { useLogsStore } from '../../store/logsStore';
import LogLine from './LogLine';

export default function LogList({ logs }) {
  const { lineWrap } = useLogsStore();

  const Row = ({ index, style }) => {
    const log = logs[index];
    return (
      <div style={style}>
        <LogLine log={log} />
      </div>
    );
  };

  if (lineWrap) {
    return (
      <div className="p-3">
        {logs.map((log) => (
          <LogLine key={log.id} log={log} />
        ))}
      </div>
    );
  }

  return (
    <List
      height={600}
      itemCount={logs.length}
      itemSize={32}
      width="100%"
      className="p-3"
    >
      {Row}
    </List>
  );
}
