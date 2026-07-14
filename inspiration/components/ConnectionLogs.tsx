"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { AuditLog, apiClient } from "@/lib/api-client";
import { AlertCircle, CheckCircle, Clock } from "lucide-react";
import { useEffect, useState } from "react";

export function ConnectionLogs({ accountId }: { accountId: string }) {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchLogs = async () => {
      try {
        const data = await apiClient.getConnectionLogs(accountId);
        setLogs(data);
      } catch (error) {
        console.error("Failed to fetch connection logs", error);
      } finally {
        setLoading(false);
      }
    };
    fetchLogs();
  }, [accountId]);

  if (loading) {
    return (
      <div className="text-sm text-zinc-500">
        {"Chargement de l'historique..."}
      </div>
    );
  }

  if (logs.length === 0) {
    return (
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm font-medium flex items-center gap-2">
            <Clock className="w-4 h-4" /> Historique de connexion
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            Aucun historique disponible.
          </p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm font-medium text-zinc-100 flex items-center gap-2">
          <Clock className="w-4 h-4" /> Historique de connexion
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          {logs.map((log) => {
            const isConnect = log.action === "connect_account";
            const date = new Date(log.timestamp);

            return (
              <div key={log.id} className="flex gap-3 items-start">
                <div
                  className={`mt-0.5 p-1 rounded-full ${isConnect ? "bg-green-500/10" : "bg-red-500/10"}`}
                >
                  {isConnect ? (
                    <CheckCircle className="w-3.5 h-3.5 text-green-500" />
                  ) : (
                    <AlertCircle className="w-3.5 h-3.5 text-red-500" />
                  )}
                </div>
                <div>
                  <p className="text-sm font-medium text-foreground">
                    {isConnect ? "Connexion réussie" : "Déconnexion"}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {new Intl.DateTimeFormat("fr-FR", {
                      dateStyle: "medium",
                      timeStyle: "medium",
                    }).format(date)}
                  </p>
                  {log.metadata && Object.keys(log.metadata).length > 0 && (
                    <p className="text-xs text-muted-foreground mt-0.5 font-mono">
                      {JSON.stringify(log.metadata)}
                    </p>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      </CardContent>
    </Card>
  );
}
