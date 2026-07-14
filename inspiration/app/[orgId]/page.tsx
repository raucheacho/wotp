"use client";

import { AccountStatusChart } from "@/components/charts/AccountStatusChart";
import { WebhookSuccessChart } from "@/components/charts/WebhookSuccessChart";

import { ExtendedStatsCard } from "@/components/ExtendedStatsCard";
import { Button } from "@/components/ui/button";
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { apiClient } from "@/lib/api-client";
import { authClient } from "@/lib/auth-client";
import { useOrgUrl } from "@/lib/url-helpers";
import type { Stats } from "@/types";
import { AlertCircle, ArrowRight, Building2, Plus, Smartphone, Webhook } from "lucide-react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

export default function OrgDashboard() {
  const router = useRouter();
  const params = useParams();
  const getUrl = useOrgUrl();
  const organizationId = params.orgId as string;
  
  const { data: session, isPending } = authClient.useSession();

  const [stats, setStats] = useState<Stats | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [orgName, setOrgName] = useState<string>("");

  // Auth Protection
  useEffect(() => {
    if (!isPending && !session) {
      router.push("/auth/sign-in");
    }
  }, [session, isPending, router]);

  const fetchData = useCallback(async () => {
    if (!organizationId) return;
    try {
      // Fetch org name and accounts stats
      const [org, accounts] = await Promise.all([
        apiClient.getOrganization(organizationId),
        apiClient.listAccounts(organizationId),
      ]);

      setOrgName(org.name);

      // Calculate stats
      const total = accounts.length;
      const connected = accounts.filter((acc) => acc.status === "connected").length;
      const disconnected = accounts.filter((acc) => acc.status === "disconnected").length;
      const errorCount = accounts.filter((acc) => acc.status === "error").length;

      // Fetch webhook delivery stats
      let webhookStats = undefined;
      try {
        webhookStats = await apiClient.getWebhookDeliveryStats(organizationId, 30);
      } catch (webhookError) {
        console.warn("Failed to fetch webhook stats:", webhookError);
      }

      setStats({
        total,
        connected,
        disconnected,
        error: errorCount,
        webhookDeliveries: webhookStats
          ? {
              total: webhookStats.total,
              successful: webhookStats.successful,
              failed: webhookStats.failed,
              averageAttempts: webhookStats.averageAttempts,
            }
          : undefined,
      });
    } catch (error: any) {
      console.error("Failed to fetch data:", error);
      setError(error.message || "Failed to load data");
    }
  }, [organizationId]);

  useEffect(() => {
    if (organizationId) {
      fetchData();
    }
  }, [organizationId, fetchData]);

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[50vh] text-center space-y-4">
        <AlertCircle className="w-12 h-12 text-destructive" />
        <div className="space-y-2">
          <h3 className="text-lg font-medium text-foreground">
            Unable to load dashboard
          </h3>
          <p className="text-muted-foreground max-w-md">{error}</p>
        </div>
        <Button onClick={() => window.location.reload()} variant="outline">
          Retry
        </Button>
      </div>
    );
  }

  if (!stats) {
    return (
      <div className="text-center py-12">
        <p className="text-muted-foreground">Chargement...</p>
      </div>
    );
  }

  return (
    <>
      {/* Header */}
      <div className="mb-8">
        <div className="flex items-center gap-4">
          <div className="p-3 bg-card rounded-xl border border-border">
            <Building2 className="w-8 h-8 text-[#25D366]" />
          </div>
          <div>
            <h1 className="text-2xl font-bold text-foreground mb-1">
              {orgName || "Tableau de bord"}
            </h1>
            <p className="text-muted-foreground text-sm">
              Vue d'ensemble de votre organisation
            </p>
          </div>
        </div>
      </div>

      {/* KPIs */}
      <div className="mb-8">
        <ExtendedStatsCard stats={stats} />
      </div>

      {/* Visual Charts */}
      <div className="mb-8">
        <h2 className="text-lg font-semibold text-foreground mb-4">Aperçu visuel</h2>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          <AccountStatusChart 
            connected={stats.connected} 
            total={stats.total} 
          />
          
          {stats.webhookDeliveries && (
            <WebhookSuccessChart 
              successful={stats.webhookDeliveries.successful}
              failed={stats.webhookDeliveries.failed}
            />
          )}
        </div>
      </div>

      {/* Quick Actions */}
      <div className="mb-8">
        <h2 className="text-lg font-semibold text-foreground mb-4">Actions rapides</h2>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          <Card className="bg-card border-border hover:border-sidebar-foreground/20 transition-colors cursor-pointer group">
            <Link href={getUrl("/accounts")}>
              <CardHeader>
                <div className="flex items-center justify-between">
                  <Smartphone className="w-5 h-5 text-[#25D366]" />
                  <ArrowRight className="w-4 h-4 text-muted-foreground group-hover:text-foreground transition-colors" />
                </div>
                <CardTitle className="text-card-foreground">Gérer les comptes</CardTitle>
                <CardDescription>
                  Ajouter, modifier ou supprimer des comptes WhatsApp
                </CardDescription>
              </CardHeader>
            </Link>
          </Card>

          <Card className="bg-card border-border hover:border-sidebar-foreground/20 transition-colors cursor-pointer group">
            <Link href={getUrl("/webhooks")}>
              <CardHeader>
                <div className="flex items-center justify-between">
                  <Webhook className="w-5 h-5 text-[#25D366]" />
                  <ArrowRight className="w-4 h-4 text-muted-foreground group-hover:text-foreground transition-colors" />
                </div>
                <CardTitle className="text-card-foreground">Webhooks</CardTitle>
                <CardDescription>
                  Consulter les stats et livraisons webhook
                </CardDescription>
              </CardHeader>
            </Link>
          </Card>

          <Card className="bg-card border-border hover:border-sidebar-foreground/20 transition-colors">
            <Link href={getUrl("/accounts")}>
              <CardHeader>
                <div className="flex items-center justify-between">
                  <Plus className="w-5 h-5 text-[#25D366]" />
                  <ArrowRight className="w-4 h-4 text-muted-foreground group-hover:text-foreground transition-colors" />
                </div>
                <CardTitle className="text-card-foreground">Connecter un compte</CardTitle>
                <CardDescription>
                  Ajouter un nouveau compte WhatsApp
                </CardDescription>
              </CardHeader>
            </Link>
          </Card>
        </div>
      </div>
    </>
  );
}
