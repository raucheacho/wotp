import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useStore } from '../store';
import { formatLogTime } from '../utils';

export default function LogsScreen() {
  const logs = useStore(state => state.logs);
  const clearLogs = useStore(state => state.clearLogs);
  const [search, setSearch] = useState('');
  const [isPaused, setIsPaused] = useState(false);
  const logBodyRef = useRef<HTMLDivElement>(null);
  const isAutoScrolling = useRef(true);

  const filteredLogs = useMemo(() => {
    if (!search.trim()) return logs;
    const query = search.toLowerCase();
    return logs.filter(
      (log) =>
        log.message.toLowerCase().includes(query) ||
        log.level.includes(query)
    );
  }, [logs, search]);

  // Auto-scroll to bottom
  useEffect(() => {
    if (!isPaused && isAutoScrolling.current && logBodyRef.current) {
      logBodyRef.current.scrollTop = logBodyRef.current.scrollHeight;
    }
  }, [filteredLogs, isPaused]);

  const handleScroll = useCallback(() => {
    if (!logBodyRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = logBodyRef.current;
    const isAtBottom = scrollHeight - scrollTop - clientHeight < 40;
    isAutoScrolling.current = isAtBottom;
    setIsPaused(!isAtBottom);
  }, []);

  const scrollToBottom = useCallback(() => {
    if (logBodyRef.current) {
      logBodyRef.current.scrollTop = logBodyRef.current.scrollHeight;
      isAutoScrolling.current = true;
      setIsPaused(false);
    }
  }, []);

  return (
    <div className="page-container">
      <div className="page-header" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <h2>Logs</h2>
          <p>Real-time system log stream</p>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <button
            className="btn btn-secondary"
            onClick={clearLogs}
          >
            Clear
          </button>
        </div>
      </div>

      <div className="log-container">
        {/* Toolbar */}
        <div className="log-toolbar">
          <span style={{ fontSize: '0.85rem' }}>🔍</span>
          <input
            type="text"
            className="input"
            placeholder="Search logs..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            style={{
              background: 'transparent',
              border: 'none',
              padding: '4px 0',
              fontSize: '0.82rem',
              flex: 1,
            }}
          />
          <span
            style={{
              fontFamily: 'var(--font-mono)',
              fontSize: '0.7rem',
              color: 'var(--text-tertiary)',
            }}
          >
            {filteredLogs.length} entries
          </span>
        </div>

        {/* Paused banner */}
        {isPaused && (
          <div className="log-paused-banner" onClick={scrollToBottom}>
            ⏸ Auto-scroll paused — click to resume
          </div>
        )}

        {/* Log body */}
        <div
          className="log-body"
          ref={logBodyRef}
          onScroll={handleScroll}
        >
          {filteredLogs.length === 0 ? (
            <div
              style={{
                padding: '60px 20px',
                textAlign: 'center',
                color: 'var(--text-tertiary)',
                fontSize: '0.82rem',
              }}
            >
              {search ? 'No logs matching your search.' : 'No log entries yet. Events will appear here in real-time.'}
            </div>
          ) : (
            filteredLogs.map((log) => (
              <div key={log.id} className="log-entry">
                <span className="log-timestamp">{formatLogTime(log.timestamp)}</span>
                <span className={`log-level ${log.level}`}>
                  {log.level.toUpperCase()}
                </span>
                <span className="log-message">{log.message}</span>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
}
