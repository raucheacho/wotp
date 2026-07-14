"use client";

import { WebhookStats } from "@/components/WebhookStats";
import { Webhook } from "lucide-react";
import { useParams } from "next/navigation";

export default function WebhooksPage() {
  const params = useParams();
  const organizationId = params.orgId as string;

  return (
    <>
      {/* Header */}
      <div className="mb-8 flex items-start justify-between">
        <div className="flex items-center gap-4">
          <div className="p-3 bg-zinc-800 rounded-xl border border-zinc-700">
            <Webhook className="w-8 h-8 text-[#25D366]" />
          </div>
          <div>
            <h1 className="text-2xl font-bold text-zinc-100 mb-1">
              Webhooks & Intégrations
            </h1>
            <p className="text-zinc-400 text-sm">
              Suivez vos livraisons webhook et leurs performances
            </p>
          </div>
        </div>
      </div>

      {/* Webhook Stats */}
      <WebhookStats organizationId={organizationId} />
    </>
  );
}
