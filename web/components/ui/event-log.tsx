export type LogLevel = "info" | "ok" | "warn" | "err";

export type LogLine = {
  time: string;
  level: LogLevel;
  message: string;
};

export default function EventLog({
  lines,
  maxHeight,
}: {
  lines: LogLine[];
  maxHeight?: number;
}) {
  return (
    <div className="log" style={maxHeight != null ? { maxHeight } : undefined}>
      {lines.length === 0 ? (
        <div>
          <span className="log-time">--:--:--</span>
          <span className="log-info">等待事件…</span>
        </div>
      ) : (
        lines.map((line, idx) => (
          <div key={idx}>
            <span className="log-time">{line.time}</span>
            <span className={`log-${line.level}`}>{line.message}</span>
          </div>
        ))
      )}
    </div>
  );
}
