import { useMemo, useState } from 'react';
import { useStore } from '../store';
import { maskPhone, timeAgo } from '../utils';
import type { MessageStatus } from '../types';
import { Clock, Check, CheckCheck, XCircle, ShieldCheck, Timer, Inbox, MessageSquare } from 'lucide-react';
import { Card } from '../components/ui/card';

const STATUS_ICONS: Record<MessageStatus, { icon: React.ReactNode; color: string }> = {
  pending: { icon: <Clock className="w-4 h-4" />, color: 'text-yellow-500' },
  sent: { icon: <Check className="w-4 h-4" />, color: 'text-blue-500' },
  delivered: { icon: <CheckCheck className="w-4 h-4" />, color: 'text-blue-500' },
  read: { icon: <CheckCheck className="w-4 h-4" />, color: 'text-[#25D366]' },
  failed: { icon: <XCircle className="w-4 h-4" />, color: 'text-destructive' },
  verified: { icon: <ShieldCheck className="w-4 h-4" />, color: 'text-[#25D366]' },
  expired: { icon: <Timer className="w-4 h-4" />, color: 'text-muted-foreground' },
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

export default function OtpEngineScreen() {
  const stats = useStore(state => state.stats);
  const messages = useStore(state => state.messages);
  const [filter, setFilter] = useState<MessageStatus | 'all'>('all');

  const filteredMessages = useMemo(() => {
    if (filter === 'all') return messages;
    return messages.filter((m) => m.status === filter);
  }, [messages, filter]);

  return (
    <div className="p-6 max-w-6xl mx-auto space-y-6">
      <div className="flex flex-col gap-2 mb-8">
        <h2 className="text-3xl font-bold tracking-tight">OTP Engine</h2>
        <p className="text-muted-foreground text-lg">Real-time OTP message stream</p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
        <Card className="p-4">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-blue-500/10 text-blue-500">
              <MessageSquare className="w-4 h-4" />
            </div>
            <div>
              <p className="text-xs text-muted-foreground">Messages Today</p>
              <p className="text-lg font-semibold text-foreground">{stats.messagesToday}</p>
            </div>
          </div>
        </Card>
        
        <Card className="p-4">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-[#25D366]/10 text-[#25D366]">
              <CheckCheck className="w-4 h-4" />
            </div>
            <div>
              <p className="text-xs text-muted-foreground">Success Rate</p>
              <p className="text-lg font-semibold text-[#25D366]">{stats.successRate}%</p>
            </div>
          </div>
        </Card>
      </div>

      <div className="flex flex-wrap gap-2 mb-4">
        {FILTER_OPTIONS.map((opt) => (
          <button
            key={opt.value}
            onClick={() => setFilter(opt.value)}
            className={`px-4 py-1.5 rounded-full text-sm font-medium transition-colors ${
              filter === opt.value
                ? 'bg-primary text-primary-foreground'
                : 'bg-muted text-muted-foreground hover:bg-muted/80'
            }`}
          >
            {opt.label}
            {opt.value !== 'all' && (
              <span className="ml-2 opacity-70">
                {messages.filter((m) => m.status === opt.value).length}
              </span>
            )}
          </button>
        ))}
      </div>

      <Card className="p-4">
        {filteredMessages.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-12 text-muted-foreground text-center">
            <Inbox className="w-12 h-12 mb-4 opacity-50" />
            <h3 className="text-lg font-semibold text-foreground mb-2">No messages yet</h3>
            <p className="max-w-sm">
              {filter === 'all'
                ? 'OTP messages will appear here in real-time as they are sent.'
                : `No messages with status "${filter}" found.`}
            </p>
          </div>
        ) : (
          <div className="space-y-2">
            {filteredMessages.map((msg) => {
              const statusInfo = STATUS_ICONS[msg.status] || { icon: <Clock className="w-4 h-4" />, color: 'text-muted-foreground' };
              return (
                <div
                  key={msg.id}
                  className="flex items-center justify-between p-3 bg-muted/30 rounded-lg hover:bg-muted/50 transition-colors"
                >
                  <div className="flex items-center gap-4 flex-1">
                    <div className="font-medium min-w-[140px]">{maskPhone(msg.phone)}</div>
                    
                    <div
                      className="font-mono text-sm px-2 py-1 rounded bg-background border text-muted-foreground tracking-widest cursor-default select-none"
                      title="For security reasons, OTP codes are hashed and cannot be revealed"
                    >
                      ••••••
                    </div>
                  </div>

                  <div className="flex items-center gap-4">
                    <div className={`flex items-center gap-1.5 ${statusInfo.color}`}>
                      {statusInfo.icon}
                      <span className="text-sm font-medium capitalize hidden sm:inline-block">
                        {msg.status}
                      </span>
                    </div>
                    
                    <div className="text-xs text-muted-foreground min-w-[80px] text-right">
                      {timeAgo(msg.sentAt)}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </Card>
    </div>
  );
}
