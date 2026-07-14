"use client";
import { AccountApiKeys } from "@/components/AccountApiKeys";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { WebhookTest } from "@/components/WebhookTest";
import { cn } from "@/lib/utils";
import type { Account } from "@/types";
import { useState } from "react";

export type AccountFormData = {
  name: string;
  webhookUrl: string;
  webhookSecret: string;
  rateLimit: number;
};

type AccountModalProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  editingAccount: Account | null;
  formData: AccountFormData;
  onFormChange: (data: AccountFormData) => void;
  onSubmit: (e: React.FormEvent) => void;
  onAccountUpdate?: (account: Account) => void;
  loading?: boolean;
};

export function AccountModal({
  open,
  onOpenChange,
  editingAccount,
  formData,
  onFormChange,
  onSubmit,
  onAccountUpdate,
  loading = false,
}: AccountModalProps) {
  const [activeTab, setActiveTab] = useState<"general" | "api">("general");

  if (!open) return null;

  const isEditing = !!editingAccount;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className={cn("max-h-[90vh] overflow-y-auto", isEditing ? "sm:max-w-3xl" : "sm:max-w-md")}>
        <DialogHeader>
          <div className="flex justify-between items-center pr-8">
            <div>
              <DialogTitle>
                {isEditing ? "Configuration du compte" : "Nouveau compte"}
              </DialogTitle>
              <DialogDescription>
                {isEditing
                  ? "Gérez les paramètres et les accès"
                  : "Configurez un nouveau compte WhatsApp"}
              </DialogDescription>
            </div>
            {isEditing && (
              <div className="flex bg-muted p-1 rounded-lg border border-border">
                <button
                  onClick={() => setActiveTab("general")}
                  className={cn(
                    "px-3 py-1.5 text-xs font-medium rounded-md transition-all",
                    activeTab === "general" 
                      ? "bg-background text-foreground shadow-sm" 
                      : "text-muted-foreground hover:text-foreground"
                  )}
                >
                  Général
                </button>
                <button
                  onClick={() => setActiveTab("api")}
                  className={cn(
                    "px-3 py-1.5 text-xs font-medium rounded-md transition-all",
                    activeTab === "api" 
                      ? "bg-background text-foreground shadow-sm" 
                      : "text-muted-foreground hover:text-foreground"
                  )}
                >
                  API & Sécurité
                </button>
              </div>
            )}
          </div>
        </DialogHeader>

        <div className="py-4">
          {(!isEditing || activeTab === "general") && (
            <form onSubmit={onSubmit}>
              <div className="space-y-4">
                <div>
                  <label className="block text-sm text-muted-foreground mb-1.5">
                    Nom du compte
                  </label>
                  <input
                    type="text"
                    value={formData.name}
                    onChange={(e) =>
                      onFormChange({ ...formData, name: e.target.value })
                    }
                    placeholder="Ex: Mon SaaS Client"
                    className="w-full px-3 py-2 bg-background border border-input rounded-lg text-sm text-foreground placeholder-muted-foreground focus:outline-none focus:border-[#25D366] transition-colors"
                    required
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
                      onFormChange({ ...formData, webhookUrl: e.target.value })
                    }
                    placeholder="https://votre-saas.com/webhook"
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
                      onFormChange({ ...formData, webhookSecret: e.target.value })
                    }
                    placeholder="Secret HMAC"
                    className="w-full px-3 py-2 bg-background border border-input rounded-lg text-sm text-foreground placeholder-muted-foreground focus:outline-none focus:border-[#25D366] transition-colors"
                  />
                </div>

                <div>
                  <label className="block text-sm text-muted-foreground mb-1.5">
                    Rate Limit <span className="text-muted-foreground/70">(msg/min)</span>
                  </label>
                  <input
                    type="number"
                    value={formData.rateLimit}
                    onChange={(e) =>
                      onFormChange({
                        ...formData,
                        rateLimit: parseInt(e.target.value) || 30,
                      })
                    }
                    min={1}
                    max={1000}
                    className="w-full px-3 py-2 bg-background border border-input rounded-lg text-sm text-foreground focus:outline-none focus:border-[#25D366] transition-colors"
                  />
                </div>
              </div>

              <DialogFooter className="mt-6">
                <button
                  type="button"
                  onClick={() => onOpenChange(false)}
                  className="px-4 py-2 text-sm rounded-lg text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
                >
                  Fermer
                </button>
                <button
                  type="submit"
                  disabled={loading}
                  className="px-4 py-2 text-sm bg-[#25D366] text-white rounded-lg hover:bg-[#1ebe5d] disabled:opacity-50 transition-colors"
                >
                  {loading ? "..." : "Enregistrer"}
                </button>
              </DialogFooter>
            </form>
          )}

          {isEditing && activeTab === "api" && (
            <div className="space-y-6">
              <AccountApiKeys 
                account={editingAccount} 
                onUpdate={(updated) => onAccountUpdate?.(updated)} 
              />
              <WebhookTest account={editingAccount} />
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
