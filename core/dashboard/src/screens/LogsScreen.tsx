import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useStore } from "../store";
import { formatLogTime } from "../utils";
import { Search, Trash2, PauseCircle } from "lucide-react";
import { Card } from "../components/ui/card";

export default function LogsScreen() {
  const logs = useStore((state) => state.logs);
  const clearLogs = useStore((state) => state.clearLogs);
  const [search, setSearch] = useState("");
  const [isPaused, setIsPaused] = useState(false);
  const logBodyRef = useRef<HTMLDivElement>(null);
  const isAutoScrolling = useRef(true);

  const filteredLogs = useMemo(() => {
    if (!search.trim()) return logs;
    const query = search.toLowerCase();
    return logs.filter(
      (log) =>
        log.message.toLowerCase().includes(query) || log.level.includes(query),
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
    <div className="p-6 max-w-6xl mx-auto space-y-6 h-[calc(100vh-2rem)] flex flex-col">
      <div className="flex justify-between items-start sm:items-center gap-4 shrink-0">
        <div className="flex flex-col gap-2">
          <h2 className="text-3xl font-bold tracking-tight">Logs</h2>
          <p className="text-muted-foreground text-lg">
            Real-time system log stream
          </p>
        </div>
        <button
          className="flex items-center gap-2 px-4 py-2 bg-muted text-foreground font-medium rounded-md hover:bg-muted/80 transition-colors"
          onClick={clearLogs}
        >
          <Trash2 className="w-4 h-4" />
          Clear
        </button>
      </div>

      <Card className="flex-1 flex flex-col min-h-0 overflow-hidden relative">
        {/* Toolbar */}
        <div className="flex items-center gap-3 p-3 border-b bg-muted/30 shrink-0">
          <Search className="w-4 h-4 text-muted-foreground" />
          <input
            type="text"
            className="flex-1 bg-transparent border-none outline-none text-sm placeholder:text-muted-foreground"
            placeholder="Search logs..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          <span className="text-xs font-mono text-muted-foreground bg-muted px-2 py-1 rounded">
            {filteredLogs.length} entries
          </span>
        </div>

        {/* Paused banner */}
        {isPaused && (
          <div
            className="absolute top-13 left-1/2 -translate-x-1/2 z-10 flex items-center gap-2 px-4 py-1.5 bg-yellow-500/90 text-yellow-950 text-sm font-medium rounded-full cursor-pointer shadow-md hover:bg-yellow-500 transition-colors"
            onClick={scrollToBottom}
          >
            <PauseCircle className="w-4 h-4" />
            Auto-scroll paused — click to resume
          </div>
        )}

        {/* Log body */}
        <div
          className="flex-1 overflow-y-auto p-4 font-mono text-sm space-y-1.5 bg-[#1e1e1e]"
          ref={logBodyRef}
          onScroll={handleScroll}
        >
          {filteredLogs.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full text-gray-500 gap-3">
              <Search className="w-8 h-8 opacity-50" />
              <p>
                {search
                  ? "No logs matching your search."
                  : "No log entries yet. Events will appear here in real-time."}
              </p>
            </div>
          ) : (
            filteredLogs.map((log) => (
              <div
                key={log.id}
                className="flex flex-col sm:flex-row sm:items-start gap-2 sm:gap-4 hover:bg-white/5 p-1 rounded transition-colors group"
              >
                <span className="text-gray-500 shrink-0 select-none">
                  [{formatLogTime(log.timestamp)}]
                </span>
                <span
                  className={`shrink-0 font-bold w-12 ${
                    log.level === "error"
                      ? "text-red-400"
                      : log.level === "warn"
                        ? "text-yellow-400"
                        : "text-blue-400"
                  }`}
                >
                  {log.level.toUpperCase()}
                </span>
                <span className="text-gray-300 break-all sm:break-words">
                  {log.message}
                </span>
              </div>
            ))
          )}
        </div>
      </Card>
    </div>
  );
}
