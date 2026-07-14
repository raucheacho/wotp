"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { apiClient } from "@/lib/api-client";
import type { WebhookDelivery } from "@/types";
import {
  AlertTriangle,
  CheckCircle,
  Clock,
  TrendingUp,
  Webhook,
  XCircle,
} from "lucide-react";
import { useEffect, useState } from "react";
import Performance from "./webhooks/Performance";

interface WebhookStatsProps {
  organizationId?: string;
}

export function WebhookStats({ organizationId }: WebhookStatsProps) {
  const [stats, setStats] = useState<{
    total: number;
    successful: number;
    failed: number;
    averageAttempts: number;
  } | null>(null);
  const [recentDeliveries, setRecentDeliveries] = useState<WebhookDelivery[]>(
    [],
  );
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  function getRateColor(rate: number) {
    if (rate >= 95) return "green";
    if (rate >= 80) return "yellow";
    return "red";
  }

  useEffect(() => {
    const fetchStats = async () => {
      if (!organizationId) return;

      try {
        setLoading(true);

        // Fetch delivery stats
        const deliveryStats = await apiClient.getWebhookDeliveryStats(
          organizationId,
          30,
        );
        setStats(deliveryStats);

        // Fetch recent deliveries
        const deliveries = await apiClient.getWebhookDeliveries({
          organizationId,
          limit: 5,
        });
        setRecentDeliveries(deliveries);

        setError(null);
      } catch (err) {
        setError(
          err instanceof Error ? err.message : "Failed to load webhook stats",
        );
      } finally {
        setLoading(false);
      }
    };

    fetchStats();
  }, [organizationId]);

  if (!organizationId) {
    return (
      <Card className="bg-card border-border">
        <div className="flex items-center gap-3 text-muted-foreground p-6">
          <Webhook className="w-5 h-5" />
          <p>
            Sélectionnez une organisation pour voir les statistiques webhooks
          </p>
        </div>
      </Card>
    );
  }

  if (loading) {
    return (
      <Card>
        <CardContent className="flex items-center justify-center py-8">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-[#25D366]"></div>
        </CardContent>
      </Card>
    );
  }

  if (error) {
    return (
      <Card>
        <CardContent className="flex items-center gap-3 text-destructive py-6">
          <AlertTriangle className="w-5 h-5" />
          <p>Erreur lors du chargement des statistiques: {error}</p>
        </CardContent>
      </Card>
    );
  }

  if (!stats) {
    return (
      <Card>
        <CardContent className="flex flex-col items-center justify-center py-8 text-muted-foreground">
          <Webhook className="w-8 h-8 mb-2" />
          <p>Aucune statistique webhook disponible</p>
        </CardContent>
      </Card>
    );
  }

  const successRate =
    stats.total > 0 ? (stats.successful / stats.total) * 100 : 0;

  return (
    <Card>
      <CardHeader className="border-b border-border pb-4">
        <div className="flex items-center justify-between">
          <div>
            <CardTitle className="flex items-center gap-2">
              <Webhook className="w-5 h-5" />
              Statistiques Webhooks
            </CardTitle>
            <p className="text-sm text-muted-foreground mt-1">
              Taux de livraison et performance des 30 derniers jours
            </p>
          </div>
        </div>
      </CardHeader>
      <CardContent className="p-6">
        {/* Métriques principales */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
          <div className="bg-muted/50 rounded-lg p-4">
            <div className="flex items-center gap-2 mb-2">
              <TrendingUp className="w-4 h-4 text-blue-500" />
              <span className="text-sm text-muted-foreground">Total</span>
            </div>
            <p className="text-2xl font-bold text-foreground">{stats.total}</p>
          </div>

          <div className="bg-muted/50 rounded-lg p-4">
            <div className="flex items-center gap-2 mb-2">
              <CheckCircle className="w-4 h-4 text-green-500" />
              <span className="text-sm text-muted-foreground">Réussis</span>
            </div>
            <p className="text-2xl font-bold text-green-500">
              {stats.successful}
            </p>
            <p className="text-xs text-muted-foreground">{successRate.toFixed(1)}%</p>
          </div>

          <div className="bg-muted/50 rounded-lg p-4">
            <div className="flex items-center gap-2 mb-2">
              <XCircle className="w-4 h-4 text-destructive" />
              <span className="text-sm text-muted-foreground">Échoués</span>
            </div>
            <p className="text-2xl font-bold text-destructive">{stats.failed}</p>
            <p className="text-xs text-muted-foreground">
              {((stats.failed / stats.total) * 100).toFixed(1)}%
            </p>
          </div>

          <div className="bg-muted/50 rounded-lg p-4">
            <div className="flex items-center gap-2 mb-2">
              <Clock className="w-4 h-4 text-yellow-500" />
              <span className="text-sm text-muted-foreground">Moy. Essais</span>
            </div>
            <p className="text-2xl font-bold text-yellow-500">
              {stats.averageAttempts.toFixed(1)}
            </p>
          </div>
        </div>

        {/* Indicateur de performance */}
        <Performance successRate={successRate} />

        {/* Livraisons récentes */}
        {recentDeliveries.length > 0 && (
          <div>
            <h4 className="text-sm font-medium text-foreground mb-3">
              Livraisons récentes
            </h4>
            <div className="space-y-2">
              {recentDeliveries.map((delivery) => (
                <div
                  key={delivery.id}
                  className="flex items-center justify-between p-3 bg-muted/30 rounded-lg"
                >
                  <div className="flex items-center gap-3">
                    {delivery.success ? (
                      <CheckCircle className="w-4 h-4 text-green-500" />
                    ) : (
                      <XCircle className="w-4 h-4 text-destructive" />
                    )}
                    <div>
                      <p className="text-sm text-foreground capitalize">
                        {delivery.event.replace("_", " ")}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        {new Date(delivery.createdAt).toLocaleString("fr-FR")}
                      </p>
                    </div>
                  </div>

                  <div className="text-right">
                    <p
                      className={`text-sm font-medium ${
                        delivery.success ? "text-green-500" : "text-destructive"
                      }`}
                    >
                      {delivery.statusCode || "Failed"}
                    </p>
                    {delivery.attempts > 1 && (
                      <p className="text-xs text-muted-foreground">
                        {delivery.attempts} essais
                      </p>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

