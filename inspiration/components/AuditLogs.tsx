"use client";

import { apiClient } from "@/lib/api-client";
import type { AuditLog } from "@/types";
import {
  AlertTriangle,
  CheckCircle,
  Clock,
  FileText,
  MoreHorizontal,
  User,
  XCircle,
} from "lucide-react";
import { useEffect, useState } from "react";

interface AuditLogsProps {
  organizationId?: string;
  limit?: number;
}

export function AuditLogs({ organizationId, limit = 10 }: AuditLogsProps) {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchLogs = async () => {
      try {
        setLoading(true);
        const auditLogs = await apiClient.getAuditLogs({
          organizationId,
          limit,
        });
        setLogs(auditLogs);
        setError(null);
      } catch (err) {
        setError(
          err instanceof Error ? err.message : "Failed to load audit logs",
        );
      } finally {
        setLoading(false);
      }
    };

    fetchLogs();
  }, [organizationId, limit]);

  const getActionIcon = (action: string) => {
    if (action.includes("create"))
      return <CheckCircle className="w-4 h-4 text-green-500" />;
    if (action.includes("delete"))
      return <XCircle className="w-4 h-4 text-red-500" />;
    if (action.includes("update"))
      return <AlertTriangle className="w-4 h-4 text-yellow-500" />;
    return <FileText className="w-4 h-4 text-blue-500" />;
  };

  const getActionLabel = (action: string) => {
    const labels: Record<string, string> = {
      create_organization: "Organisation créée",
      update_organization: "Organisation modifiée",
      delete_organization: "Organisation supprimée",
      create_account: "Compte créé",
      update_account: "Compte modifié",
      delete_account: "Compte supprimé",
      send_message: "Message envoyé",
      connect_account: "Connexion établie",
      disconnect_account: "Déconnexion",
      generate_qr: "QR généré",
      login_attempt: "Tentative de connexion",
      api_access: "Accès API",
    };
    return labels[action] || action.replace(/_/g, " ");
  };

  const formatTimestamp = (timestamp: string) => {
    return new Date(timestamp).toLocaleString("fr-FR", {
      day: "2-digit",
      month: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  if (loading) {
    return (
      <div className="bg-card border border-border rounded-xl p-6">
        <div className="flex items-center justify-center py-8">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-[#25D366]"></div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-card border border-border rounded-xl p-6">
        <div className="flex items-center gap-3 text-destructive">
          <AlertTriangle className="w-5 h-5" />
          <p>
            {"Erreur lors du chargement des logs d'audit"}: {error}
          </p>
        </div>
      </div>
    );
  }

  if (logs.length === 0) {
    return (
      <div className="bg-card border border-border rounded-xl p-6">
        <div className="flex items-center justify-center py-8 text-muted-foreground">
          <FileText className="w-8 h-8 mb-2" />
          <p>{"Aucun log d'audit trouvé"}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-card border border-border rounded-xl">
      <div className="px-6 py-4 border-b border-border">
        <h3 className="font-semibold text-card-foreground flex items-center gap-2">
          <FileText className="w-5 h-5" />
          {" Logs d'Audit"}
        </h3>
        <p className="text-sm text-muted-foreground mt-1">
          Historique des actions administratives
        </p>
      </div>

      <div className="divide-y divide-border">
        {logs.map((log) => (
          <div
            key={log.id}
            className="px-6 py-4 hover:bg-muted/50 transition-colors"
          >
            <div className="flex items-start gap-4">
              <div className="p-2 rounded-lg bg-muted">
                {getActionIcon(log.action)}
              </div>

              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 mb-1">
                  <p className="font-medium text-foreground">
                    {getActionLabel(log.action)}
                  </p>
                  {log.resourceName && (
                    <span className="text-sm text-muted-foreground">
                      • {log.resourceName}
                    </span>
                  )}
                  {!log.success && (
                    <span className="px-2 py-0.5 text-xs bg-destructive/10 text-destructive rounded-full">
                      Échoué
                    </span>
                  )}
                </div>

                <div className="flex items-center gap-4 text-sm text-muted-foreground mb-2">
                  <div className="flex items-center gap-1">
                    <Clock className="w-3 h-3" />
                    {formatTimestamp(log.timestamp)}
                  </div>

                  {log.ipAddress && (
                    <div className="flex items-center gap-1">
                      <User className="w-3 h-3" />
                      {log.ipAddress}
                    </div>
                  )}

                  {log.userAgent && (
                    <div className="text-xs truncate max-w-xs">
                      {log.userAgent.split(" ")[0]}...
                    </div>
                  )}
                </div>

                {log.errorMessage && (
                  <div className="bg-destructive/10 border border-destructive/20 rounded-lg p-3 mt-2">
                    <p className="text-sm text-destructive">
                      {log.errorMessage}
                    </p>
                  </div>
                )}

                {log.oldValues && log.newValues && (
                  <div className="mt-2 text-xs text-muted-foreground">
                    <p>Modifications détectées</p>
                  </div>
                )}
              </div>

              <div className="flex items-center gap-1 text-muted-foreground">
                <MoreHorizontal className="w-4 h-4" />
              </div>
            </div>
          </div>
        ))}
      </div>

      {logs.length >= limit && (
        <div className="px-6 py-3 border-t border-border bg-muted/30">
          <p className="text-sm text-muted-foreground text-center">
            Affichage des {limit} derniers logs •{" "}
            <button className="text-[#25D366] hover:underline">
              Voir tout
            </button>
          </p>
        </div>
      )}
    </div>
  );
}
