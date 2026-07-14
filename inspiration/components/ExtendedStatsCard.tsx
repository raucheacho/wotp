"use client";

import type { Stats } from "@/types";
import {
  Activity,
  AlertTriangle,
  CheckCircle2,
  Smartphone,
} from "lucide-react";
import StatCard from "./partials/StatCard";

interface ExtendedStatsCardProps {
  stats: Stats;
}

export function ExtendedStatsCard({ stats }: ExtendedStatsCardProps) {
  // Calculate webhook success rate
  const webhookSuccessRate = stats.webhookDeliveries 
    ? Math.round((stats.webhookDeliveries.successful / (stats.webhookDeliveries.successful + stats.webhookDeliveries.failed)) * 100)
    : 0;

  return (
    <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
      {/* Total Accounts */}
      <StatCard
        title="Total"
        value={stats.total}
        icon={
          <div className="p-1 rounded-lg bg-secondary">
            <Smartphone className="w-4 h-4 text-muted-foreground" />
          </div>
        }
      />

      {/* Active (Connected) Accounts */}
      <StatCard
        title="Actifs"
        value={stats.connected}
        icon={
          <div className="p-1 rounded-lg bg-[#25D366]/10">
            <Activity className="w-4 h-4 text-[#25D366]" />
          </div>
        }
      />

      {/* Errors */}
      <StatCard
        title="Erreurs"
        value={stats.error}
        icon={
          <div className="p-1 rounded-lg bg-destructive/10">
            <AlertTriangle className="w-4 h-4 text-destructive" />
          </div>
        }
      />

      {/* Webhook Success Rate */}
      {stats.webhookDeliveries && (
        <StatCard
          title="Webhooks"
          value={`${webhookSuccessRate}%`}
          icon={
            <div className="p-1 rounded-lg bg-[#25D366]/10">
              <CheckCircle2 className="w-4 h-4 text-[#25D366]" />
            </div>
          }
        />
      )}
    </div>
  );
}
