"use client";
import type { Account } from "@/types";
import { statusConfig } from "@/types";
import {
  Activity,
  AlertCircle,
  CheckCircle,
  Clock,
  Webhook,
  Zap,
} from "lucide-react";

const statusIcons = {
  connected: CheckCircle,
  connecting: Clock,
  disconnected: AlertCircle,
  error: AlertCircle,
} as const;

type AccountInfoCardsProps = {
  account: Account;
  messagesCount: number;
};

export function AccountInfoCards({
  account,
  messagesCount,
}: AccountInfoCardsProps) {
  const status = statusConfig[account.status] || statusConfig.disconnected;
  const StatusIcon = statusIcons[account.status as keyof typeof statusIcons] || AlertCircle;

  return (
    <div className="grid grid-cols-4 gap-4 mb-6">
      <div className="bg-card border border-border rounded-xl p-4">
        <div className="flex items-center gap-3">
          <div className={`p-2 rounded-lg ${status.bg}`}>
            <StatusIcon className={`w-4 h-4 ${status.color}`} />
          </div>
          <div>
            <p className="text-xs text-muted-foreground">Status</p>
            <p className="font-medium text-card-foreground">{status.label}</p>
          </div>
        </div>
      </div>

      <div className="bg-card border border-border rounded-xl p-4">
        <div className="flex items-center gap-3">
          <div className="p-2 rounded-lg bg-purple-500/10">
            <Zap className="w-4 h-4 text-purple-500" />
          </div>
          <div>
            <p className="text-xs text-muted-foreground">Rate Limit</p>
            <p className="font-medium text-card-foreground">
              {account.rateLimit || 30}/min
            </p>
          </div>
        </div>
      </div>

      <div className="bg-card border border-border rounded-xl p-4">
        <div className="flex items-center gap-3">
          <div
            className={`p-2 rounded-lg ${
              account.webhookUrl ? "bg-[#25D366]/10" : "bg-muted"
            }`}
          >
            <Webhook
              className={`w-4 h-4 ${
                account.webhookUrl ? "text-[#25D366]" : "text-muted-foreground"
              }`}
            />
          </div>
          <div>
            <p className="text-xs text-muted-foreground">Webhook</p>
            <p className="font-medium text-card-foreground">
              {account.webhookUrl ? "Actif" : "Non configuré"}
            </p>
          </div>
        </div>
      </div>

      <div className="bg-card border border-border rounded-xl p-4">
        <div className="flex items-center gap-3">
          <div className="p-2 rounded-lg bg-blue-500/10">
            <Activity className="w-4 h-4 text-blue-500" />
          </div>
          <div>
            <p className="text-xs text-muted-foreground">Messages</p>
            <p className="font-medium text-card-foreground">{messagesCount}</p>
          </div>
        </div>
      </div>
    </div>
  );
}
