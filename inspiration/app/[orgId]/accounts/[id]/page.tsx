"use client";
import { AccountInfoCards } from "@/components/AccountInfoCards";
import { CodeSnippetsModal } from "@/components/CodeSnippetsModal";
import { ConnectionLogs } from "@/components/ConnectionLogs";
import { MessageLogs } from "@/components/MessageLogs";
import { VisualQuota } from "@/components/VisualQuota";
import { apiClient } from "@/lib/api-client";
import { useOrgUrl } from "@/lib/url-helpers";
import { useAppStore } from "@/stores/useAppStore";
import type { Account, MessageError, MessageLog } from "@/types";
import { statusConfig } from "@/types";
import { AlertCircle, ArrowLeft, Code2, RefreshCw } from "lucide-react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

export default function AccountDetailPage() {
  const params = useParams();
  const router = useRouter();
  const getUrl = useOrgUrl();
  const accountId = params.id as string;
  const { setCurrentAccount, setRefreshing, setOnRefresh } = useAppStore();

  const [account, setAccount] = useState<Account | null>(null);
  const [logs, setLogs] = useState<MessageLog[]>([]);
  const [errors, setErrors] = useState<MessageError[]>([]);
  const [activeTab, setActiveTab] = useState<"logs" | "errors">("logs");
  const [loading, setLoading] = useState(true);
  const [showSnippets, setShowSnippets] = useState(false);

  const fetchAll = useCallback(async () => {
    try {
      const [accountData, logsData, errorsData] = await Promise.all([
        apiClient.getAccount(accountId),
        apiClient.getMessageLogs(accountId, { limit: 50 }),
        apiClient.getMessageErrors(accountId, 50),
      ]);

      setAccount(accountData);
      setCurrentAccount(accountData);
      setLogs(logsData.logs || []);
      setErrors(errorsData.errors || []);
    } catch (e) {
      console.error(e);
      // Optional: Handle error state more explicitly if needed
    } finally {
      setLoading(false);
    }
  }, [accountId, setCurrentAccount]);

  useEffect(() => {
    void fetchAll();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [accountId]);

  const refresh = useCallback(async () => {
    setRefreshing(true);
    await fetchAll();
    setRefreshing(false);
  }, [fetchAll, setRefreshing]);

  // Configurer le callback de refresh dans le store
  useEffect(() => {
    setOnRefresh(() => refresh);
  }, [refresh, setOnRefresh]);

  // Nettoyer le compte courant quand on quitte la page
  useEffect(() => {
    return () => setCurrentAccount(undefined);
  }, [setCurrentAccount]);

    if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <RefreshCw className="w-6 h-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (!account) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-center">
          <AlertCircle className="w-10 h-10 mx-auto mb-3 text-destructive" />
          <p className="text-foreground">Compte non trouvé</p>
          <Link
            href={getUrl("/")}
            className="mt-3 text-sm text-[#25D366] hover:underline inline-block"
          >
            Retour
          </Link>
        </div>
      </div>
    );
  }

  const status = statusConfig[account.status] || statusConfig.disconnected;

  return (
    <>
      <div className="flex items-center gap-3 mb-6">
        <Link
          href={getUrl("/")}
          className="p-2 rounded-lg hover:bg-muted text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="w-4 h-4" />
        </Link>
        <div>
          <h1 className="text-xl font-semibold text-foreground">
            {account.name}
          </h1>
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <span className={status.color}>{status.label}</span>
            {account.phoneNumber && (
              <>
                <span>•</span>
                <span>{account.phoneNumber}</span>
              </>
            )}
          </div>
        </div>
        <div className="ml-auto">
          <button
            onClick={() => setShowSnippets(true)}
            className="p-2 rounded-lg hover:bg-muted text-muted-foreground hover:text-foreground transition-colors"
            title="Exemples de code"
          >
            <Code2 className="w-5 h-5" />
          </button>
        </div>
      </div>

      <AccountInfoCards account={account} messagesCount={logs.length} />

      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-6">
        <div className="md:col-span-1">
           <VisualQuota account={account} />
        </div>
        <div className="md:col-span-2">
           <ConnectionLogs accountId={account.id} />
        </div>
      </div>

      {account.webhookUrl && (
        <div className="bg-muted/50 border border-border rounded-xl p-4 mb-6">
          <p className="text-xs text-muted-foreground mb-1">Webhook URL</p>
          <code className="text-sm text-[#25D366]">{account.webhookUrl}</code>
        </div>
      )}

      <MessageLogs
        logs={logs}
        errors={errors}
        activeTab={activeTab}
        onTabChange={setActiveTab}
      />
      <CodeSnippetsModal
        open={showSnippets}
        onClose={() => setShowSnippets(false)}
        account={account}
      />
    </>
  );
}
