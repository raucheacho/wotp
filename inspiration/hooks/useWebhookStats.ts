import { apiClient, type WebhookDelivery } from "@/lib/api-client";
import { type Stats } from "@/types";
import { useEffect, useState } from "react";

interface WebhookStatsState {
  stats: NonNullable<Stats['webhookDeliveries']> | null;
  deliveries: WebhookDelivery[];
  loading: boolean;
  error: string | null;
}

function useWebhookStats(organizationId?: string) {
  const [state, setState] = useState<WebhookStatsState>({
    stats: null,
    deliveries: [],
    loading: true,
    error: null,
  });

  useEffect(() => {
    if (!organizationId) return;

    (async () => {
      try {
        const [stats, deliveries] = await Promise.all([
          apiClient.getWebhookDeliveryStats(organizationId, 30),
          apiClient.getWebhookDeliveries({ organizationId, limit: 5 }),
        ]);

        setState({ stats, deliveries, loading: false, error: null });
      } catch (e) {
        setState((s) => ({
          ...s,
          loading: false,
          error: e instanceof Error ? e.message : "Erreur",
        }));
      }
    })();
  }, [organizationId]);

  return state;
}
