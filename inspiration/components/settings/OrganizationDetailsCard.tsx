"use client";

import { Organization } from "@/types";
import { Check, Copy, Key, Shield, Webhook, Zap } from "lucide-react";
import { useState } from "react";
import { QuotaIndicator } from "../QuotaIndicator";

interface OrganizationDetailsCardProps {
  organization: Organization;
}

export function OrganizationDetailsCard({
  organization,
}: OrganizationDetailsCardProps) {
  const [copiedKey, setCopiedKey] = useState(false);

  const handleCopyApiKey = async () => {
    if (organization?.apiKey) {
      await navigator.clipboard.writeText(organization.apiKey);
      setCopiedKey(true);
      setTimeout(() => setCopiedKey(false), 2000);
    }
  };

  return (
    <div className="bg-card border border-border rounded-xl overflow-hidden mb-6">
      <div className="px-6 py-4 border-b border-border flex justify-between items-center bg-muted/20">
        <h3 className="font-semibold text-foreground flex items-center gap-2">
          <Shield className="w-5 h-5 text-[#25D366]" />
          {"Configuration de l'organisation"}
        </h3>
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground uppercase tracking-wider font-medium">
            Plan:
          </span>
          <span className="text-xs px-2 py-1 bg-muted text-foreground rounded-full border border-border capitalize">
            {organization.plan}
          </span>
        </div>
      </div>

      <div className="p-6 flex flex-col gap-6">
        {/* API Key Section */}
        <div className="space-y-2">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Key className="w-4 h-4" />
            <span>Clé API</span>
          </div>
          <div className="flex items-center gap-2 bg-muted/50 border border-border rounded-lg p-2 group">
            <code className="flex-1 font-mono text-xs text-foreground truncate">
              {organization.apiKey || "Aucune clé générée"}
            </code>
            <button
              onClick={handleCopyApiKey}
              className="p-1.5 hover:bg-muted/80 rounded-md transition-colors text-muted-foreground hover:text-foreground"
              title="Copier la clé"
            >
              {copiedKey ? (
                <Check className="w-4 h-4 text-green-500" />
              ) : (
                <Copy className="w-4 h-4" />
              )}
            </button>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-4">
          {/* Webhook Section */}
          <div className="space-y-2">
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Webhook className="w-4 h-4" />
              <span>Webhook</span>
            </div>
            <div className="flex items-center gap-2 p-2 px-0">
              <div
                className={`w-2 h-2 rounded-full ${organization.webhookUrl ? "bg-green-500" : "bg-muted-foreground"}`}
              />
              <span
                className={`text-sm font-medium ${organization.webhookUrl ? "text-foreground" : "text-muted-foreground"}`}
              >
                {organization.webhookUrl ? "Actif" : "Inactif"}
              </span>
            </div>
          </div>

          {/* Rate Limit Section */}
          <div className="space-y-2">
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Zap className="w-4 h-4" />
              <span>Rate Limit</span>
            </div>
            <div className="flex items-center gap-2 p-2 px-0">
              <span className="text-xl font-semibold text-foreground">
                {organization.defaultRateLimit}
              </span>
              <span className="text-sm text-muted-foreground">req/min</span>
            </div>
          </div>
        </div>
      </div>

      {/* Quota Section embedded at bottom */}
      <div className="px-6 py-4 bg-muted/30 border-t border-border">
        <QuotaIndicator organizationId={organization.id} compact />
      </div>
    </div>
  );
}
