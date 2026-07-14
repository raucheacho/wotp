"use client";

import type { Organization } from "@/types";
import { Building2, Key, Trash2, Webhook } from "lucide-react";
import Link from "next/link";

type OrganizationListProps = {
  organizations: Organization[];
  onDelete: (id: string) => void;
};

export function OrganizationList({
  organizations,
  onDelete,
}: OrganizationListProps) {
  if (organizations.length === 0) {
    return (
      <div className="bg-card border border-border rounded-xl p-8 text-center">
        <Building2 className="w-12 h-12 text-muted-foreground mx-auto mb-3" />
        <p className="text-muted-foreground text-sm">
          Aucune organization. Créez-en une pour commencer.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {organizations.map((org) => (
        <div
          key={org.id}
          className="bg-card border border-border rounded-xl p-4 hover:border-sidebar-foreground/20 transition-colors"
        >
          <div className="flex items-start justify-between">
            <div className="flex-1">
              <div className="flex items-center gap-3 mb-2">
                <Building2 className="w-5 h-5 text-[#25D366]" />
                <h3 className="font-semibold text-card-foreground">{org.name}</h3>
                <span
                  className={`px-2 py-0.5 text-xs rounded-full ${
                    org.plan === "enterprise"
                      ? "bg-purple-500/10 text-purple-400"
                      : org.plan === "pro"
                      ? "bg-blue-500/10 text-blue-400"
                      : "bg-muted text-muted-foreground"
                  }`}
                >
                  {org.plan}
                </span>
              </div>

              <div className="grid grid-cols-2 gap-4 text-sm">
                <div className="flex items-center gap-2 text-muted-foreground">
                  <Key className="w-4 h-4" />
                  <span className="font-mono text-xs truncate">
                    {org.apiKey.substring(0, 20)}...
                  </span>
                </div>

                {org.webhookUrl && (
                  <div className="flex items-center gap-2 text-muted-foreground">
                    <Webhook className="w-4 h-4" />
                    <span className="text-xs truncate">{org.webhookUrl}</span>
                  </div>
                )}

                <div className="text-muted-foreground">
                  <span className="text-muted-foreground/70">Comptes:</span>{" "}
                  <span className="text-foreground">
                    {(org.metadata as Record<string, unknown>)?.accountsUsed as number || 0}/{org.maxAccounts}
                  </span>
                </div>

                <div className="text-muted-foreground">
                  <span className="text-muted-foreground/70">Rate limit:</span>{" "}
                  <span className="text-foreground">
                    {org.defaultRateLimit} msg/min
                  </span>
                </div>
              </div>
            </div>

            <div className="flex items-center gap-2">
              <Link
                href={`/organizations/${org.id}`}
                className="px-3 py-1.5 text-sm text-muted-foreground hover:text-foreground hover:bg-muted rounded-lg transition-colors"
              >
                Voir comptes
              </Link>
              <button
                onClick={() => onDelete(org.id)}
                className="p-2 text-destructive hover:text-destructive/80 hover:bg-destructive/10 rounded-lg transition-colors"
              >
                <Trash2 className="w-4 h-4" />
              </button>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
