"use client";

import { AccountList } from "@/components/AccountList";
import { AccountModal, type AccountFormData } from "@/components/AccountModal";
import { Button } from "@/components/ui/button";
import { apiClient } from "@/lib/api-client";
import type { Account } from "@/types";
import { Plus, Smartphone } from "lucide-react";
import { useParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

const defaultFormData: AccountFormData = {
  name: "",
  webhookUrl: "",
  webhookSecret: "",
  rateLimit: 30,
};

export default function AccountsPage() {
  const params = useParams();
  const organizationId = params.orgId as string;

  const [accounts, setAccounts] = useState<Account[]>([]);
  const [showAccountModal, setShowAccountModal] = useState(false);
  const [loading, setLoading] = useState(false);
  const [formData, setFormData] = useState<AccountFormData>(defaultFormData);
  const [editingAccountId, setEditingAccountId] = useState<string | null>(null);

  const fetchAccounts = useCallback(async () => {
    if (!organizationId) return;
    try {
      const data = await apiClient.listAccounts(organizationId);
      setAccounts(data);
    } catch (error) {
      console.error("Failed to fetch accounts:", error);
    }
  }, [organizationId]);

  useEffect(() => {
    if (organizationId) {
      fetchAccounts();
    }
  }, [organizationId, fetchAccounts]);

  const handleSubmitAccount = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!formData.name.trim() || !organizationId) return;

    setLoading(true);
    try {
      if (editingAccountId) {
        await apiClient.updateAccount(editingAccountId, {
          name: formData.name,
          webhookUrl: formData.webhookUrl || null,
          webhookSecret: formData.webhookSecret || undefined,
          rateLimit: formData.rateLimit,
        });
      } else {
        await apiClient.createAccount({
          organizationId,
          name: formData.name,
          webhookUrl: formData.webhookUrl || undefined,
          webhookSecret: formData.webhookSecret || undefined,
          rateLimit: formData.rateLimit,
        });
      }

      setFormData(defaultFormData);
      setEditingAccountId(null);
      setShowAccountModal(false);
      await fetchAccounts();
    } catch (error) {
      console.error("Failed to save account:", error);
      alert(
        `Erreur lors de la sauvegarde: ${error instanceof Error ? error.message : "Erreur inconnue"}`,
      );
    }
    setLoading(false);
  };

  const handleEdit = (account: Account) => {
    setEditingAccountId(account.id);
    setFormData({
      name: account.name,
      webhookUrl: account.webhookUrl || "",
      webhookSecret: "",
      rateLimit: account.rateLimit || 30,
    });
    setShowAccountModal(true);
  };

  const handleDelete = async (id: string) => {
    if (!confirm("Supprimer ce compte ?")) return;
    try {
      await apiClient.deleteAccount(id);
      await fetchAccounts();
    } catch (error) {
      console.error("Failed to delete account:", error);
      alert(
        `Erreur lors de la suppression: ${error instanceof Error ? error.message : "Erreur inconnue"}`,
      );
    }
  };

  const handleAccountUpdate = (updatedAccount: Account) => {
    setAccounts((prev) =>
      prev.map((acc) => (acc.id === updatedAccount.id ? updatedAccount : acc)),
    );
  };

  return (
    <>
      {/* Header */}
      <div className="mb-8 flex items-start justify-between">
        <div className="flex items-center gap-4">
          <div className="p-3 bg-zinc-800 rounded-xl border border-zinc-700">
            <Smartphone className="w-8 h-8 text-[#25D366]" />
          </div>
          <div>
            <h1 className="text-2xl font-bold text-zinc-100 mb-1">
              Comptes WhatsApp
            </h1>
            <p className="text-zinc-400 text-sm">
              Gérez vos comptes WhatsApp connectés
            </p>
          </div>
        </div>
        <Button
          onClick={() => setShowAccountModal(true)}
          className="bg-[#25D366] hover:bg-[#1ebe5d] text-black font-medium gap-2"
        >
          <Plus className="w-4 h-4" />
          Connecter un compte
        </Button>
      </div>

      {/* Account List */}
      <AccountList
        accounts={accounts}
        onEdit={handleEdit}
        onDelete={handleDelete}
      />

      {/* Account Modal */}
      <AccountModal
        open={showAccountModal}
        onOpenChange={(open) => {
          if (!open) {
            setFormData(defaultFormData);
            setEditingAccountId(null);
          }
          setShowAccountModal(open);
        }}
        editingAccount={
          editingAccountId
            ? accounts.find((a) => a.id === editingAccountId) || null
            : null
        }
        formData={formData}
        onFormChange={setFormData}
        onSubmit={handleSubmitAccount}
        onAccountUpdate={handleAccountUpdate}
        loading={loading}
      />
    </>
  );
}
