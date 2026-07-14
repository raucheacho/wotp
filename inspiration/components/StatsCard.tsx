"use client";
import { Card } from "@/components/ui/card";
import type { Stats } from "@/types";
import {
  Activity,
  AlertCircle,
  CheckCircle,
  Clock,
  LucideIcon,
} from "lucide-react";

type StatItemProps = {
  icon: LucideIcon;
  iconBg: string;
  iconColor: string;
  value: number;
  label: string;
};

function StatItem({
  icon: Icon,
  iconBg,
  iconColor,
  value,
  label,
}: StatItemProps) {
  return (
    <Card className="p-4">
      <div className="flex items-center gap-3">
        <div className={`p-2 rounded-lg ${iconBg}`}>
          <Icon className={`w-4 h-4 ${iconColor}`} />
        </div>
        <div>
          <p className="text-xl font-semibold text-foreground">{value}</p>
          <p className="text-xs text-muted-foreground">{label}</p>
        </div>
      </div>
    </Card>
  );
}

type StatsCardsProps = {
  stats: Stats;
};

export function StatsCards({ stats }: StatsCardsProps) {
  return (
    <div className="grid grid-cols-4 gap-4 mb-6">
      <StatItem
        icon={Activity}
        iconBg="bg-blue-500/10"
        iconColor="text-blue-500"
        value={stats.total}
        label="Total"
      />
      <StatItem
        icon={CheckCircle}
        iconBg="bg-[#25D366]/10"
        iconColor="text-[#25D366]"
        value={stats.connected}
        label="Connectés"
      />
      <StatItem
        icon={Clock}
        iconBg="bg-zinc-700/50"
        iconColor="text-zinc-400"
        value={stats.disconnected}
        label="Déconnectés"
      />
      <StatItem
        icon={AlertCircle}
        iconBg="bg-red-500/10"
        iconColor="text-red-500"
        value={stats.error}
        label="Erreurs"
      />
    </div>
  );
}
