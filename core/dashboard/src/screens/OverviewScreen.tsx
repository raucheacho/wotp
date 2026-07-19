import { useCallback, useEffect, useState } from 'react';
import { useStore } from '../store';
import { Card } from '../components/ui/card';
import { Zap, CheckCircle, Activity, Cloud, Smartphone, AlertTriangle } from 'lucide-react';
import type { WaNumber, CloudStatus } from '../types';

// Overview is the default landing screen — it should answer "is this
// project healthy right now," not greet a returning operator with a
// marketing hero and a copy-pasteable cURL example. That belongs in the
// README, not the screen someone checks every day.
export default function OverviewScreen() {
  const stats = useStore((state) => state.stats);
  const selectedProjectId = useStore((state) => state.selectedProjectId);
  const [number, setNumber] = useState<WaNumber | null>(null);
  const [cloud, setCloud] = useState<CloudStatus | null>(null);

  const fetchStatus = useCallback(async () => {
    if (!selectedProjectId) return;
    try {
      const res = await fetch(
        `/dashboard/api/numbers?project_id=${encodeURIComponent(selectedProjectId)}`,
      );
      if (res.ok) {
        const numbers: WaNumber[] = await res.json();
        setNumber(numbers[0] ?? null);
      }
    } catch {
      // next poll will retry
    }
    try {
      const res = await fetch(
        `/dashboard/api/cloud-status?project_id=${encodeURIComponent(selectedProjectId)}`,
      );
      if (res.ok) setCloud(await res.json());
    } catch {
      // next poll will retry
    }
  }, [selectedProjectId]);

  useEffect(() => {
    fetchStatus();
    const interval = setInterval(fetchStatus, 10000);
    return () => clearInterval(interval);
  }, [fetchStatus]);

  const whatsmeowOk = !!number?.connected;
  const cloudEnabled = !!cloud?.enabled;
  const cloudOk = cloudEnabled && !!cloud?.connected;
  const canSendOtp = whatsmeowOk || cloudOk;

  return (
    <div className="p-6 max-w-6xl mx-auto space-y-6">
      <div className="flex flex-col gap-2 mb-2">
        <h2 className="text-3xl font-bold tracking-tight">Overview</h2>
        <p className="text-muted-foreground text-lg">This project's health at a glance.</p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Card className="p-6">
          <div className="flex items-center gap-4">
            <div
              className={`p-3 rounded-full flex items-center justify-center ${
                whatsmeowOk ? 'bg-[#25D366]/10 text-[#25D366]' : 'bg-muted text-muted-foreground'
              }`}
            >
              <Smartphone className="w-6 h-6" />
            </div>
            <div>
              <h3 className="text-lg font-semibold">WhatsApp number</h3>
              <p className="text-muted-foreground text-sm">
                {number
                  ? whatsmeowOk
                    ? `Connected — ${number.phone}`
                    : `Paired but disconnected — ${number.phone}`
                  : 'No number paired yet'}
              </p>
            </div>
          </div>
        </Card>

        <Card className="p-6">
          <div className="flex items-center gap-4">
            <div
              className={`p-3 rounded-full flex items-center justify-center ${
                !cloudEnabled
                  ? 'bg-muted text-muted-foreground'
                  : cloudOk
                    ? 'bg-[#25D366]/10 text-[#25D366]'
                    : 'bg-destructive/10 text-destructive'
              }`}
            >
              <Cloud className="w-6 h-6" />
            </div>
            <div>
              <h3 className="text-lg font-semibold">Cloud API (OTP)</h3>
              <p className="text-muted-foreground text-sm">
                {!cloudEnabled
                  ? 'Not enabled for this project'
                  : cloudOk
                    ? `Connected — ${cloud?.display_phone || cloud?.phone_number_id}`
                    : 'Enabled, but credentials could not be verified'}
              </p>
            </div>
          </div>
        </Card>
      </div>

      {!canSendOtp && (
        <Card className="p-4 border-destructive/50">
          <div className="flex items-center gap-3 text-destructive">
            <AlertTriangle className="w-5 h-5 shrink-0" />
            <p className="font-medium text-sm">
              This project can't send OTPs right now — pair a WhatsApp number or enable the Cloud
              API backend in Settings.
            </p>
          </div>
        </Card>
      )}

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card className="p-4">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-blue-500/10 text-blue-500">
              <Zap className="w-4 h-4" />
            </div>
            <div>
              <p className="text-xs text-muted-foreground">Total OTPs Today</p>
              <p className="text-lg font-semibold text-foreground">{stats.messagesToday}</p>
            </div>
          </div>
        </Card>

        <Card className="p-4">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-[#25D366]/10 text-[#25D366]">
              <CheckCircle className="w-4 h-4" />
            </div>
            <div>
              <p className="text-xs text-muted-foreground">Delivery Success</p>
              <p className="text-lg font-semibold text-foreground">{stats.successRate}%</p>
            </div>
          </div>
        </Card>

        <Card className="p-4">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-yellow-500/10 text-yellow-500">
              <Activity className="w-4 h-4" />
            </div>
            <div>
              <p className="text-xs text-muted-foreground">Avg Latency</p>
              <p className="text-lg font-semibold text-foreground">
                {stats.avgResponseMs > 0 ? `${stats.avgResponseMs}ms` : '< 1s'}
              </p>
            </div>
          </div>
        </Card>
      </div>
    </div>
  );
}
