"use client";

import { AlertTriangle, CheckCircle, Users } from "lucide-react";
import { useEffect, useState } from "react";

interface QuotaIndicatorProps {
  organizationId: string;
  compact?: boolean;
}

export function QuotaIndicator({ organizationId, compact = false }: QuotaIndicatorProps) {
  const [usage, setUsage] = useState<{
    used: number;
    max: number;
    available: number;
    percentage: number;
  } | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchUsage = async () => {
      try {
        setLoading(true);
        const response = await fetch(
          `/api/organizations/${organizationId}/usage`,
        );
        if (response.ok) {
          const data = await response.json();
          setUsage(data.usage);
          setError(null);
        } else {
            console.warn("Failed to fetch quota")
        }
      } catch (err) {
        console.error("Failed to fetch quota usage:", err);
      } finally {
        setLoading(false);
      }
    };

    fetchUsage();
    const interval = setInterval(fetchUsage, 30000);
    return () => clearInterval(interval);
  }, [organizationId]);

  if (loading || error || !usage) return null;

  const isNearLimit = usage.percentage >= 80;
  const isAtLimit = usage.percentage >= 100;

  if (compact) {
      return (
          <div className="flex items-center gap-4 w-full">
              <div className="flex items-center gap-2 min-w-fit">
                    <Users className={`w-4 h-4 ${isAtLimit ? 'text-destructive' : 'text-muted-foreground'}`} />
                    <span className="text-sm font-medium text-muted-foreground">Quota: {usage.used} / {usage.max}</span>
              </div>
              <div className="flex-1 h-1.5 bg-muted rounded-full overflow-hidden">
                   <div
                    className={`h-full rounded-full transition-all duration-300 ${
                    isAtLimit
                        ? "bg-destructive"
                        : isNearLimit
                        ? "bg-yellow-500"
                        : "bg-[#25D366]"
                    }`}
                    style={{ width: `${Math.min(usage.percentage, 100)}%` }}
                />
              </div>
          </div>
      )
  }

  return (
    <div
      className={`bg-card border rounded-xl p-4 ${
        isAtLimit
          ? "border-destructive/50"
          : isNearLimit
            ? "border-yellow-500/50"
            : "border-border"
      }`}
    >
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <Users
            className={`w-5 h-5 ${
              isAtLimit
                ? "text-destructive"
                : isNearLimit
                  ? "text-yellow-500"
                  : "text-[#25D366]"
            }`}
          />
          <h3 className="font-semibold text-foreground">Quota de comptes</h3>
        </div>
        {isAtLimit ? (
          <AlertTriangle className="w-5 h-5 text-destructive" />
        ) : isNearLimit ? (
          <AlertTriangle className="w-5 h-5 text-yellow-500" />
        ) : (
          <CheckCircle className="w-5 h-5 text-green-500" />
        )}
      </div>

      <div className="space-y-2">
        <div className="flex items-center justify-between text-sm">
          <span className="text-muted-foreground">Utilisés</span>
          <span className="font-medium text-foreground">
            {usage.used} / {usage.max}
          </span>
        </div>

        <div className="w-full bg-muted rounded-full h-2">
          <div
            className={`h-2 rounded-full transition-all duration-300 ${
              isAtLimit
                ? "bg-destructive"
                : isNearLimit
                  ? "bg-yellow-500"
                  : "bg-[#25D366]"
            }`}
            style={{ width: `${Math.min(usage.percentage, 100)}%` }}
          />
        </div>

        <div className="flex items-center justify-between text-xs">
          <span className="text-muted-foreground">{usage.available} disponibles</span>
          <span
            className={`font-medium ${
              isAtLimit
                ? "text-destructive"
                : isNearLimit
                  ? "text-yellow-500"
                  : "text-muted-foreground"
            }`}
          >
            {usage.percentage}%
          </span>
        </div>

        {isAtLimit && (
          <div className="mt-3 p-3 bg-destructive/10 border border-destructive/20 rounded-lg">
            <p className="text-sm text-destructive">
              ⚠️ Quota atteint! Vous ne pouvez plus créer de comptes.
            </p>
          </div>
        )}

        {isNearLimit && !isAtLimit && (
          <div className="mt-3 p-3 bg-yellow-500/10 border border-yellow-500/20 rounded-lg">
            <p className="text-sm text-yellow-500">
              ⚠️ Quota presque atteint ({usage.available} comptes restants)
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
