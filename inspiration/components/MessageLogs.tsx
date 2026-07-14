"use client";
import { Card } from "@/components/ui/card";
import type { MessageError, MessageLog } from "@/types";
import { logStatusConfig } from "@/types";
import { AlertCircle, CheckCircle, MessageSquare, Send } from "lucide-react";

type MessageLogsProps = {
  logs: MessageLog[];
  errors: MessageError[];
  activeTab: "logs" | "errors";
  onTabChange: (tab: "logs" | "errors") => void;
};

export function MessageLogs({
  logs,
  errors,
  activeTab,
  onTabChange,
}: MessageLogsProps) {
  return (
    <Card className="overflow-hidden">
      <div className="flex border-b border-border">
        <button
          onClick={() => onTabChange("logs")}
          className={`px-4 py-2.5 text-sm border-b-2 -mb-px transition-colors ${
            activeTab === "logs"
              ? "border-[#25D366] text-[#25D366]"
              : "border-transparent text-muted-foreground hover:text-foreground"
          }`}
        >
          <MessageSquare className="w-4 h-4 inline mr-2" />
          Messages ({logs.length})
        </button>
        <button
          onClick={() => onTabChange("errors")}
          className={`px-4 py-2.5 text-sm border-b-2 -mb-px transition-colors ${
            activeTab === "errors"
              ? "border-destructive text-destructive"
              : "border-transparent text-muted-foreground hover:text-foreground"
          }`}
        >
          <AlertCircle className="w-4 h-4 inline mr-2" />
          Erreurs ({errors.length})
        </button>
      </div>

      {activeTab === "logs" && (
        <div className="divide-y divide-border">
          {logs.length === 0 ? (
            <div className="px-6 py-12 text-center">
              <MessageSquare className="w-8 h-8 mx-auto mb-2 text-muted-foreground" />
              <p className="text-muted-foreground text-sm">Aucun message</p>
            </div>
          ) : (
            logs.map((log) => {
              const ls = logStatusConfig[log.status] || logStatusConfig.queued;
              return (
                <div key={log.id} className="px-4 py-3 hover:bg-muted/50">
                  <div className="flex items-center gap-3">
                    <div
                      className={`p-1.5 rounded-lg ${
                        log.direction === "sent"
                          ? "bg-blue-500/10"
                          : "bg-[#25D366]/10"
                      }`}
                    >
                      {log.direction === "sent" ? (
                        <Send className="w-3.5 h-3.5 text-blue-500" />
                      ) : (
                        <MessageSquare className="w-3.5 h-3.5 text-[#25D366]" />
                      )}
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 text-sm">
                        <span className="text-foreground">{log.phoneNumber}</span>
                        <span className="text-xs px-1.5 py-0.5 rounded bg-muted text-muted-foreground">
                          {log.messageType}
                        </span>
                      </div>
                      {log.contentPreview && (
                        <p className="text-xs text-muted-foreground truncate">
                          {log.contentPreview}
                        </p>
                      )}
                    </div>
                    <div className="text-right">
                      <span className={`text-xs ${ls.color}`}>{ls.label}</span>
                      <p className="text-[10px] text-muted-foreground">
                        {new Date(log.timestamp).toLocaleString("fr-FR")}
                      </p>
                    </div>
                  </div>
                </div>
              );
            })
          )}
        </div>
      )}

      {activeTab === "errors" && (
        <div className="divide-y divide-border">
          {errors.length === 0 ? (
            <div className="px-6 py-12 text-center">
              <CheckCircle className="w-8 h-8 mx-auto mb-2 text-muted-foreground" />
              <p className="text-muted-foreground text-sm">Aucune erreur</p>
            </div>
          ) : (
            errors.map((err) => (
              <div key={err.id} className="px-4 py-3 hover:bg-muted/50">
                <div className="flex items-center gap-3">
                  <div className="p-1.5 rounded-lg bg-destructive/10">
                    <AlertCircle className="w-3.5 h-3.5 text-destructive" />
                  </div>
                  <div className="flex-1">
                    <span className="text-sm text-destructive">
                      {err.errorCode}
                    </span>
                    <p className="text-xs text-muted-foreground">{err.errorMessage}</p>
                  </div>
                  <p className="text-[10px] text-muted-foreground">
                    {new Date(err.timestamp).toLocaleString("fr-FR")}
                  </p>
                </div>
              </div>
            ))
          )}
        </div>
      )}
    </Card>
  );
}
