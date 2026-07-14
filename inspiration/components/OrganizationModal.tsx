"use client";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useState } from "react";

type OrganizationModalProps = {
  open: boolean;
  onClose: () => void;
  onSubmit: (data: {
    name: string;
    webhookUrl?: string;
    webhookSecret?: string;
    plan?: "starter" | "pro" | "enterprise";
    maxAccounts?: number;
  }) => void;
  loading?: boolean;
};

export function OrganizationModal({
  open,
  onClose,
  onSubmit,
  loading = false,
}: OrganizationModalProps) {
  const [formData, setFormData] = useState<{
    name: string;
    webhookUrl: string;
    webhookSecret: string;
    plan: "starter" | "pro" | "enterprise";
    maxAccounts: number;
  }>({
    name: "",
    webhookUrl: "",
    webhookSecret: "",
    plan: "starter",
    maxAccounts: 10,
  });

  if (!open) return null;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onSubmit({
      name: formData.name,
      webhookUrl: formData.webhookUrl || undefined,
      webhookSecret: formData.webhookSecret || undefined,
      plan: formData.plan,
      maxAccounts: formData.maxAccounts,
    });
  };

  return (
    <Dialog open={open} onOpenChange={(val) => !val && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader >
          <DialogTitle>Nouvelle Organization</DialogTitle>
          <DialogDescription>
            Créez une organization pour un client SaaS
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit}>
          <div className="space-y-4 py-4">
            <div>
              <label className="block text-sm text-muted-foreground mb-1.5">
                Nom de l'organization
              </label>
              <input
                type="text"
                value={formData.name}
                onChange={(e) =>
                  setFormData({ ...formData, name: e.target.value })
                }
                placeholder="Ex: Mon SaaS Client"
                className="w-full px-3 py-2 bg-background border border-input rounded-lg text-sm text-foreground placeholder-muted-foreground focus:outline-none focus:border-[#25D366] transition-colors"
                required
              />
            </div>

            <div>
              <label className="block text-sm text-muted-foreground mb-1.5">Plan</label>
              <select
                value={formData.plan}
                onChange={(e) =>
                  setFormData({ ...formData, plan: e.target.value as "starter" | "pro" | "enterprise" })
                }
                className="w-full px-3 py-2 bg-background border border-input rounded-lg text-sm text-foreground focus:outline-none focus:border-[#25D366] transition-colors"
              >
                <option value="starter">Starter (10 comptes)</option>
                <option value="pro">Pro (100 comptes)</option>
                <option value="enterprise">Enterprise (500 comptes)</option>
              </select>
            </div>

            <div>
              <label className="block text-sm text-muted-foreground mb-1.5">
                Limite de comptes
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
                max={1000}
                className="w-full px-3 py-2 bg-background border border-input rounded-lg text-sm text-foreground focus:outline-none focus:border-[#25D366] transition-colors"
              />
            </div>

            <div>
              <label className="block text-sm text-muted-foreground mb-1.5">
                Webhook URL <span className="text-muted-foreground/70">(optionnel)</span>
              </label>
              <input
                type="url"
                value={formData.webhookUrl}
                onChange={(e) =>
                  setFormData({ ...formData, webhookUrl: e.target.value })
                }
                placeholder="https://client-saas.com/webhook"
                className="w-full px-3 py-2 bg-background border border-input rounded-lg text-sm text-foreground placeholder-muted-foreground focus:outline-none focus:border-[#25D366] transition-colors"
              />
            </div>

            <div>
              <label className="block text-sm text-muted-foreground mb-1.5">
                Webhook Secret{" "}
                <span className="text-muted-foreground/70">(optionnel)</span>
              </label>
              <input
                type="password"
                value={formData.webhookSecret}
                onChange={(e) =>
                  setFormData({ ...formData, webhookSecret: e.target.value })
                }
                placeholder="Secret HMAC"
                className="w-full px-3 py-2 bg-background border border-input rounded-lg text-sm text-foreground placeholder-muted-foreground focus:outline-none focus:border-[#25D366] transition-colors"
              />
            </div>
          </div>

          <DialogFooter>
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm rounded-lg text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
            >
              Annuler
            </button>
            <button
              type="submit"
              disabled={loading}
              className="px-4 py-2 text-sm bg-[#25D366] text-white rounded-lg hover:bg-[#1ebe5d] disabled:opacity-50 transition-colors"
            >
              {loading ? "..." : "Créer"}
            </button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
