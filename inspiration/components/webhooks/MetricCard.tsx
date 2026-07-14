import { LucideIcon } from "lucide-react";

interface MetricCardProps {
  icon: LucideIcon | React.ElementType;
  label: string;
  value: string | number;
  color?: string;
  footer?: React.ReactNode;
}

export function MetricCard({ icon: Icon, label, value, color, footer }: MetricCardProps) {
  return (
    <div className="bg-zinc-800/50 rounded-lg p-4">
      <div className="flex items-center gap-2 mb-2">
        <Icon className={`w-4 h-4 ${color}`} />
        <span className="text-sm text-zinc-400">{label}</span>
      </div>
      <p className={`text-2xl font-bold ${color}`}>{value}</p>
      {footer}
    </div>
  );
}
