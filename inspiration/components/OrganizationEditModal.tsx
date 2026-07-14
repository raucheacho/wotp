"use client";

import type { Organization, OrganizationPlan } from "@/types";
import { X } from "lucide-react";
import { useState } from "react";

export interface OrganizationFormData {
  name: string;
  webhookUrl: string;
  webhookSecret: string;
  plan: OrganizationPlan;
  maxAccounts: number;
  defaultRateLimit: number;
  webhookFilter: {
    events: string[];
    includeOwnMessages: boolean;
  };
}

type OrganizationEditModalProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  organization: Organization | null;
  onSave: (data: OrganizationFormData) => Promise<void>;
  loading: boolean;
};

const planOptions: { value: OrganizationPlan; label: string }[] = [
  { value: "starter", label: "Starter" },
  { value: "pro", label: "Pro" },
  { value: "enterprise", label: "Enterprise" },
];

const eventOptions = [
  { value: "message.received", label: "Messages reçus" },
  { value: "message.sent", label: "Messages envoyés" },
  { value: "message.failed", label: "Messages échoués" },
  { value: "account.connected", label: "Compte connecté" },
  { value: "account.disconnected", label: "Compte déconnecté" },
];

const defaultFilter = {
  events: ["message.received", "account.connected", "account.disconnected"],
  includeOwnMessages: false,
};

function getInitialFormData(
  organization: Organization | null
): OrganizationFormData {
  if (!organization) {
    return {
      name: "",
      webhookUrl: "",
      webhookSecret: "",
      plan: "starter",
      maxAccounts: 10,
      defaultRateLimit: 30,
      webhookFilter: defaultFilter,
    };
  }

  const metadata = organization.metadata as Record<string, unknown> | null;
  const savedFilter = metadata?.webhookFilter as
    | typeof defaultFilter
    | undefined;

  return {
    name: organization.name,
    webhookUrl: organization.webhookUrl || "",
    webhookSecret: "",
    plan: organization.plan as OrganizationPlan,
    maxAccounts: organization.maxAccounts,
    defaultRateLimit: organization.defaultRateLimit,
    webhookFilter: savedFilter || defaultFilter,
  };
}

export function OrganizationEditModal({
  open,
  onOpenChange,
  organization,
  onSave,
  loading,
}: OrganizationEditModalProps) {
  const [formData, setFormData] = useState<OrganizationFormData>(() =>
    getInitialFormData(organization)
  );

  // Reset form data when organization changes
  const [prevOrgId, setPrevOrgId] = useState<string | null>(
    organization?.id ?? null
  );
  if (organization?.id !== prevOrgId) {
    setPrevOrgId(organization?.id ?? null);
    setFormData(getInitialFormData(organization));
  }

  if (!open) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    await onSave(formData);
  };

  const toggleEvent = (event: string) => {
    const events = formData.webhookFilter.events.includes(event)
      ? formData.webhookFilter.events.filter((e) => e !== event)
      : [...formData.webhookFilter.events, event];
    setFormData({
      ...formData,
      webhookFilter: { ...formData.webhookFilter, events },
    });
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div
        className="absolute inset-0 bg-background/80 backdrop-blur-sm"
        onClick={() => onOpenChange(false)}
      />
      <div className="relative bg-card border border-border rounded-xl w-full max-w-lg mx-4 shadow-2xl max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between px-5 py-4 border-b border-border sticky top-0 bg-card z-10">
          <h2 className="text-lg font-medium text-card-foreground">
            Modifier l&apos;organisation
          </h2>
          <button
            onClick={() => onOpenChange(false)}
            className="p-1 rounded-lg hover:bg-muted text-muted-foreground hover:text-foreground"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-5 space-y-4">
          <div>
            <label className="block text-sm text-muted-foreground mb-1.5">Nom</label>
            <input
              type="text"
              value={formData.name}
              onChange={(e) =>
                setFormData({ ...formData, name: e.target.value })
              }
              className="w-full px-3 py-2 bg-background border border-input rounded-lg text-foreground text-sm focus:outline-none focus:border-[#25D366]"
              required
            />
          </div>

          <div>
            <label className="block text-sm text-muted-foreground mb-1.5">Plan</label>
            <select
              value={formData.plan}
              onChange={(e) =>
                setFormData({
                  ...formData,
                  plan: e.target.value as OrganizationPlan,
                })
              }
              className="w-full px-3 py-2 bg-background border border-input rounded-lg text-foreground text-sm focus:outline-none focus:border-[#25D366]"
            >
              {planOptions.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm text-muted-foreground mb-1.5">
                Max comptes
              </label>
              <input
                type="number"
                value={formData.maxAccounts}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    maxAccounts: parseInt(e.target.value) || 10,
                  })
                }
                min={1}
                className="w-full px-3 py-2 bg-background border border-input rounded-lg text-foreground text-sm focus:outline-none focus:border-[#25D366]"
              />
            </div>
            <div>
              <label className="block text-sm text-muted-foreground mb-1.5">
                Rate limit
              </label>
              <input
                type="number"
                value={formData.defaultRateLimit}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    defaultRateLimit: parseInt(e.target.value) || 30,
                  })
                }
                min={1}
                max={1000}
                className="w-full px-3 py-2 bg-background border border-input rounded-lg text-foreground text-sm focus:outline-none focus:border-[#25D366]"
              />
            </div>
          </div>

          <div>
            <label className="block text-sm text-muted-foreground mb-1.5">
              Webhook URL
            </label>
            <input
              type="url"
              value={formData.webhookUrl}
              onChange={(e) =>
                setFormData({ ...formData, webhookUrl: e.target.value })
              }
              placeholder="https://..."
              className="w-full px-3 py-2 bg-background border border-input rounded-lg text-foreground text-sm focus:outline-none focus:border-[#25D366]"
            />
          </div>

          <div>
            <label className="block text-sm text-muted-foreground mb-1.5">
              Webhook Secret
            </label>
            <input
              type="password"
              value={formData.webhookSecret}
              onChange={(e) =>
                setFormData({ ...formData, webhookSecret: e.target.value })
              }
              placeholder="Laisser vide pour ne pas modifier"
              className="w-full px-3 py-2 bg-background border border-input rounded-lg text-foreground text-sm focus:outline-none focus:border-[#25D366]"
            />
          </div>

          {/* Event Filters */}
          <div className="border-t border-border pt-4">
            <label className="block text-sm font-medium text-foreground mb-3">
              Filtres d&apos;événements webhook
            </label>

            <div className="space-y-2 mb-4">
              {eventOptions.map((opt) => (
                <label
                  key={opt.value}
                  className="flex items-center gap-3 p-2 rounded-lg hover:bg-muted cursor-pointer"
                >
                  <input
                    type="checkbox"
                    checked={formData.webhookFilter.events.includes(opt.value)}
                    onChange={() => toggleEvent(opt.value)}
                    className="w-4 h-4 rounded border-input bg-background text-[#25D366] focus:ring-[#25D366]"
                  />
                  <span className="text-sm text-muted-foreground">{opt.label}</span>
                </label>
              ))}
            </div>

            <div className="space-y-2 border-t border-border pt-3">
              <label className="flex items-center gap-3 p-2 rounded-lg hover:bg-muted cursor-pointer">
                <input
                  type="checkbox"
                  checked={formData.webhookFilter.includeOwnMessages}
                  onChange={(e) =>
                    setFormData({
                      ...formData,
                      webhookFilter: {
                        ...formData.webhookFilter,
                        includeOwnMessages: e.target.checked,
                      },
                    })
                  }
                  className="w-4 h-4 rounded border-input bg-background text-[#25D366] focus:ring-[#25D366]"
                />
                <span className="text-sm text-muted-foreground">
                  Inclure mes propres messages
                </span>
              </label>
            </div>
          </div>

          <div className="flex gap-3 pt-2">
            <button
              type="button"
              onClick={() => onOpenChange(false)}
              className="flex-1 px-4 py-2 bg-muted text-muted-foreground text-sm rounded-lg hover:bg-secondary"
            >
              Annuler
            </button>
            <button
              type="submit"
              disabled={loading || !formData.name.trim()}
              className="flex-1 px-4 py-2 bg-[#25D366] text-white text-sm rounded-lg hover:bg-[#1ebe5d] disabled:opacity-50"
            >
              {loading ? "..." : "Enregistrer"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
