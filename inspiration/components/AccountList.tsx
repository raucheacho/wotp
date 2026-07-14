"use client";
import type { Account } from "@/types";
import { statusConfig } from "@/types";
import {
  Activity,
  AlertCircle,
  CheckCircle,
  ChevronRight,
  Clock,
  ExternalLink,
  MessageSquare,
  Settings,
  Trash2,
  Webhook,
  Zap,
} from "lucide-react";
import Link from "next/link";

const statusIcons = {
  connected: CheckCircle,
  connecting: Clock,
  disconnected: AlertCircle,
  error: AlertCircle,
};

type AccountListProps = {
  accounts: Account[];
  onEdit: (account: Account) => void;
  onDelete: (id: string) => void;
};

export function AccountList({ accounts, onEdit, onDelete }: AccountListProps) {
  if (accounts.length === 0) {
    return (
      <div className="bg-card border border-border rounded-xl overflow-hidden">
        <div className="px-4 py-3 border-b border-border">
          <h2 className="text-sm font-medium text-card-foreground">
            Tous les comptes
          </h2>
        </div>
        <div className="px-6 py-16 text-center">
          <div className="w-12 h-12 rounded-full bg-muted flex items-center justify-center mx-auto mb-4">
            <MessageSquare className="w-6 h-6 text-muted-foreground" />
          </div>
          <p className="text-muted-foreground mb-1">Aucun compte</p>
          <p className="text-sm text-muted-foreground/70">
            Créez votre premier compte WhatsApp
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-card border border-border rounded-xl overflow-hidden">
      <div className="px-4 py-3 border-b border-border">
        <h2 className="text-sm font-medium text-card-foreground">Tous les comptes</h2>
      </div>
      <div className="divide-y divide-border">
        {accounts.map((account) => {
          const status =
            statusConfig[account.status] || statusConfig.disconnected;
          const StatusIcon = statusIcons[account.status] || AlertCircle;

          return (
            <div
              key={account.id}
              className="px-4 py-3 hover:bg-muted/50 transition-colors group"
            >
              <div className="flex items-center gap-4">
                <div className={`p-2 rounded-lg ${status.bg}`}>
                  <StatusIcon className={`w-4 h-4 ${status.color}`} />
                </div>

                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-foreground text-sm">
                      {account.name}
                    </span>
                    {account.webhookUrl && (
                      <Webhook className="w-3 h-3 text-[#25D366]" />
                    )}
                  </div>
                  <div className="flex items-center gap-2 text-xs text-muted-foreground mt-0.5">
                    <span className={status.color}>{status.label}</span>
                    {account.phoneNumber && (
                      <>
                        <span className="text-muted-foreground/70">•</span>
                        <span>{account.phoneNumber}</span>
                      </>
                    )}
                    <span className="text-muted-foreground/70">•</span>
                    <span className="flex items-center gap-1">
                      <Zap className="w-3 h-3" />
                      {account.rateLimit || 30}/min
                    </span>
                  </div>
                </div>

                <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                  <a
                    href={`/connect/${account.publicToken}`}
                    target="_blank"
                    className="p-1.5 rounded-lg hover:bg-muted text-muted-foreground hover:text-foreground"
                    title="Scanner QR"
                  >
                    <ExternalLink className="w-4 h-4" />
                  </a>
                  <Link
                    href={`/accounts/${account.id}`}
                    className="p-1.5 rounded-lg hover:bg-muted text-muted-foreground hover:text-foreground"
                    title="Détails"
                  >
                    <Activity className="w-4 h-4" />
                  </Link>
                  <button
                    onClick={() => onEdit(account)}
                    className="p-1.5 rounded-lg hover:bg-muted text-muted-foreground hover:text-foreground"
                    title="Modifier"
                  >
                    <Settings className="w-4 h-4" />
                  </button>
                  <button
                    onClick={() => onDelete(account.id)}
                    className="p-1.5 rounded-lg hover:bg-destructive/10 text-muted-foreground hover:text-destructive"
                    title="Supprimer"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>

                <ChevronRight className="w-4 h-4 text-muted-foreground/50" />
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
