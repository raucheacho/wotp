import { useMemo, useState } from 'react';
import { useStore } from '../store';
import { maskPhone, timeAgo } from '../utils';
import type { MessageStatus } from '../types';
import {
  Clock,
  Check,
  CheckCheck,
  XCircle,
  ShieldCheck,
  Timer,
  Inbox,
  FileText,
  Image as ImageIcon,
  File,
  LayoutTemplate,
} from 'lucide-react';
import { Card } from '../components/ui/card';

// Activity merges what used to be two separate screens (OTP Engine,
// Messaging API) — they were both just "outbound messages sent through this
// project," filtered differently. One feed, one type toggle, matches how an
// operator actually thinks about it: "what did this project send recently."
type Kind = 'all' | 'otp' | 'message';

const STATUS_ICONS: Record<MessageStatus, { icon: React.ReactNode; color: string }> = {
  pending: { icon: <Clock className="w-4 h-4" />, color: 'text-yellow-500' },
  sent: { icon: <Check className="w-4 h-4" />, color: 'text-blue-500' },
  delivered: { icon: <CheckCheck className="w-4 h-4" />, color: 'text-blue-500' },
  read: { icon: <CheckCheck className="w-4 h-4" />, color: 'text-[#25D366]' },
  failed: { icon: <XCircle className="w-4 h-4" />, color: 'text-destructive' },
  verified: { icon: <ShieldCheck className="w-4 h-4" />, color: 'text-[#25D366]' },
  expired: { icon: <Timer className="w-4 h-4" />, color: 'text-muted-foreground' },
};

const TYPE_ICONS: Record<string, React.ReactNode> = {
  text: <FileText className="w-3 h-3" />,
  image: <ImageIcon className="w-3 h-3" />,
  document: <File className="w-3 h-3" />,
  template: <LayoutTemplate className="w-3 h-3" />,
};

interface UnifiedItem {
  id: string;
  kind: 'otp' | 'message';
  phone: string;
  status: MessageStatus;
  sentAt: string;
  msgType?: string;
  content?: string;
}

export default function ActivityScreen() {
  const otpMessages = useStore((state) => state.messages);
  const genericMessages = useStore((state) => state.genericMessages);
  const [kindFilter, setKindFilter] = useState<Kind>('all');
  const [statusFilter, setStatusFilter] = useState<MessageStatus | 'all'>('all');

  const items = useMemo<UnifiedItem[]>(() => {
    const otp: UnifiedItem[] = otpMessages.map((m) => ({
      id: `otp-${m.id}`,
      kind: 'otp',
      phone: m.phone,
      status: m.status,
      sentAt: m.sentAt,
    }));
    const generic: UnifiedItem[] = genericMessages.map((m) => ({
      id: `msg-${m.id}`,
      kind: 'message',
      phone: m.to,
      status: m.status,
      sentAt: m.sentAt,
      msgType: m.type,
      content: m.content,
    }));
    return [...otp, ...generic].sort(
      (a, b) => new Date(b.sentAt).getTime() - new Date(a.sentAt).getTime(),
    );
  }, [otpMessages, genericMessages]);

  const filtered = useMemo(() => {
    return items.filter((item) => {
      if (kindFilter !== 'all' && item.kind !== kindFilter) return false;
      if (statusFilter !== 'all' && item.status !== statusFilter) return false;
      return true;
    });
  }, [items, kindFilter, statusFilter]);

  const failedToday = items.filter((i) => i.status === 'failed').length;

  return (
    <div className="p-6 max-w-6xl mx-auto space-y-6">
      <div className="flex flex-col gap-2 mb-2">
        <h2 className="text-3xl font-bold tracking-tight">Activity</h2>
        <p className="text-muted-foreground text-lg">
          Everything this project has sent — OTP codes and generic messages, in one feed.
        </p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-2">
        <Card className="p-4">
          <p className="text-xs text-muted-foreground">Total sends (loaded)</p>
          <p className="text-lg font-semibold text-foreground">{items.length}</p>
        </Card>
        <Card className="p-4">
          <p className="text-xs text-muted-foreground">OTP sends</p>
          <p className="text-lg font-semibold text-foreground">{otpMessages.length}</p>
        </Card>
        <Card className="p-4">
          <p className="text-xs text-muted-foreground">Failed</p>
          <p className={`text-lg font-semibold ${failedToday > 0 ? 'text-destructive' : 'text-foreground'}`}>
            {failedToday}
          </p>
        </Card>
      </div>

      <div className="flex flex-wrap items-center gap-4">
        <div className="flex gap-1 p-1 bg-muted rounded-lg">
          {(['all', 'otp', 'message'] as Kind[]).map((k) => (
            <button
              key={k}
              onClick={() => setKindFilter(k)}
              className={`px-3 py-1 rounded-md text-sm font-medium capitalize transition-colors ${
                kindFilter === k ? 'bg-background shadow-sm text-foreground' : 'text-muted-foreground'
              }`}
            >
              {k === 'otp' ? 'OTP' : k}
            </button>
          ))}
        </div>

        <div className="flex flex-wrap gap-2">
          {(['all', 'sent', 'delivered', 'read', 'verified', 'failed', 'expired'] as const).map((s) => (
            <button
              key={s}
              onClick={() => setStatusFilter(s)}
              className={`px-3 py-1 rounded-full text-xs font-medium transition-colors ${
                statusFilter === s
                  ? 'bg-primary text-primary-foreground'
                  : 'bg-muted text-muted-foreground hover:bg-muted/80'
              }`}
            >
              {s}
            </button>
          ))}
        </div>
      </div>

      <Card className="p-4">
        {filtered.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-12 text-muted-foreground text-center">
            <Inbox className="w-12 h-12 mb-4 opacity-50" />
            <h3 className="text-lg font-semibold text-foreground mb-2">No activity yet</h3>
            <p className="max-w-sm">Sends will appear here in real-time as they go out.</p>
          </div>
        ) : (
          <div className="space-y-2">
            {filtered.map((item) => {
              const statusInfo = STATUS_ICONS[item.status] || {
                icon: <Clock className="w-4 h-4" />,
                color: 'text-muted-foreground',
              };
              return (
                <div
                  key={item.id}
                  className="flex items-center justify-between p-3 bg-muted/30 rounded-lg hover:bg-muted/50 transition-colors gap-4"
                >
                  <div className="flex items-center gap-4 flex-1 min-w-0">
                    <span className="text-[10px] font-semibold uppercase tracking-wider px-1.5 py-0.5 rounded bg-muted text-muted-foreground shrink-0">
                      {item.kind === 'otp' ? 'OTP' : (item.msgType ?? 'message')}
                    </span>
                    <div className="font-medium min-w-[140px]">{maskPhone(item.phone)}</div>
                    {item.kind === 'otp' ? (
                      <div
                        className="font-mono text-sm px-2 py-1 rounded bg-background border text-muted-foreground tracking-widest cursor-default select-none"
                        title="For security reasons, OTP codes are hashed and cannot be revealed"
                      >
                        ••••••
                      </div>
                    ) : (
                      <div className="flex items-center gap-1.5 text-sm text-muted-foreground truncate">
                        {item.msgType && TYPE_ICONS[item.msgType]}
                        <span className="truncate">{item.content}</span>
                      </div>
                    )}
                  </div>

                  <div className="flex items-center gap-4 shrink-0">
                    <div className={`flex items-center gap-1.5 ${statusInfo.color}`}>
                      {statusInfo.icon}
                      <span className="text-sm font-medium capitalize hidden sm:inline-block">
                        {item.status}
                      </span>
                    </div>
                    <div className="text-xs text-muted-foreground min-w-[80px] text-right">
                      {timeAgo(item.sentAt)}
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
