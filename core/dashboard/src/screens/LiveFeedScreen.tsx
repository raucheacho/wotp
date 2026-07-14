import { useMemo, useState } from 'react';
import { useStore } from '../store';
import { maskPhone, timeAgo } from '../utils';
import type { MessageStatus } from '../types';

const STATUS_ICONS: Record<MessageStatus, { icon: string; label: string }> = {
  pending: { icon: '◌', label: 'Pending' },
  sent: { icon: '✓', label: 'Sent' },
  delivered: { icon: '✓✓', label: 'Delivered' },
  read: { icon: '✓✓', label: 'Read' },
  failed: { icon: '✗', label: 'Failed' },
  verified: { icon: '★', label: 'Verified' },
  expired: { icon: '⏱', label: 'Expired' },
};

const FILTER_OPTIONS: Array<{ value: MessageStatus | 'all'; label: string }> = [
  { value: 'all', label: 'All' },
  { value: 'sent', label: 'Sent' },
  { value: 'delivered', label: 'Delivered' },
  { value: 'read', label: 'Read' },
  { value: 'verified', label: 'Verified' },
  { value: 'failed', label: 'Failed' },
  { value: 'expired', label: 'Expired' },
];

export default function LiveFeedScreen() {
  const stats = useStore(state => state.stats);
  const messages = useStore(state => state.messages);
  const [filter, setFilter] = useState<MessageStatus | 'all'>('all');
  const [revealedCodes, setRevealedCodes] = useState<Set<string>>(new Set());

  const filteredMessages = useMemo(() => {
    if (filter === 'all') return messages;
    return messages.filter((m) => m.status === filter);
  }, [messages, filter]);

  const toggleCode = (id: string) => {
    setRevealedCodes((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  return (
    <div className="page-container">
      <div className="page-header">
        <h2>Live Feed</h2>
        <p>Real-time OTP message stream</p>
      </div>

      {/* Stats Bar */}
      <div className="stats-bar">
        <div className="stat-card">
          <div className="stat-label">Messages Today</div>
          <div className="stat-value">{stats.messagesToday}</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Success Rate</div>
          <div className="stat-value accent">{stats.successRate}%</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Avg Response</div>
          <div className="stat-value">
            {stats.avgResponseMs > 0 ? `${stats.avgResponseMs}ms` : '—'}
          </div>
        </div>
      </div>

      {/* Filter Bar */}
      <div className="filter-bar">
        {FILTER_OPTIONS.map((opt) => (
          <button
            key={opt.value}
            className={`filter-chip ${filter === opt.value ? 'active' : ''}`}
            onClick={() => setFilter(opt.value)}
          >
            {opt.label}
            {opt.value !== 'all' && (
              <span style={{ marginLeft: 4, opacity: 0.7 }}>
                {messages.filter((m) => m.status === opt.value).length}
              </span>
            )}
          </button>
        ))}
      </div>

      {/* Message List */}
      {filteredMessages.length === 0 ? (
        <div className="empty-state">
          <div className="empty-icon">📨</div>
          <h3>No messages yet</h3>
          <p>
            {filter === 'all'
              ? 'OTP messages will appear here in real-time as they are sent.'
              : `No messages with status "${filter}" found.`}
          </p>
        </div>
      ) : (
        <div className="message-list" style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          {filteredMessages.map((msg, i) => {
            const statusInfo = STATUS_ICONS[msg.status] || { icon: '?', label: msg.status };
            const isRevealed = revealedCodes.has(msg.id);

            return (
              <div
                key={msg.id}
                className="message-row"
                style={{ animationDelay: `${Math.min(i * 30, 300)}ms` }}
              >
                {/* Phone */}
                <div className="message-phone">{maskPhone(msg.phone)}</div>

                {/* Code */}
                <div
                  className={`message-code ${isRevealed ? 'revealed' : ''}`}
                  onClick={() => toggleCode(msg.id)}
                  title={isRevealed ? 'Click to hide' : 'Click to reveal code'}
                >
                  {isRevealed ? msg.code : '••••••'}
                </div>

                {/* Status */}
                <div className={`message-status ${msg.status}`}>
                  <span>{statusInfo.icon}</span>
                </div>

                {/* Time */}
                <div className="message-time">{timeAgo(msg.sentAt)}</div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
