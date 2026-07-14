"use client";

import { AuditLogs } from "@/components/AuditLogs";
import { useParams } from "next/navigation";

export default function LogsPage() {
  const params = useParams();
  const orgId = params.orgId as string;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Logs de l'organisation</h1>
        <p className="text-muted-foreground">
          Consultez l'historique des activités et des événements de l'organisation.
        </p>
      </div>

      <AuditLogs organizationId={orgId} limit={50} />
    </div>
  );
}
