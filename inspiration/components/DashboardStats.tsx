"use client";

import type { Account } from "@/lib/api-client";
import { apiClient } from "@/lib/api-client";
import { useOrganizationStore } from "@/stores/store";
import { Building2, Smartphone, Zap } from "lucide-react";
import { useEffect, useState } from "react";

interface StatProps {
  label: string;
  value: string | number;
  icon: React.ReactNode;
  colorFn: (val: any) => string;
}

import { Card } from "@/components/ui/card";

function StatCard({ label, value, icon, colorFn }: StatProps) {
  return (
    <Card className="p-4">
      <div className="flex items-center gap-3">
        <div className={`p-2 rounded-lg ${colorFn(value)} bg-opacity-10`}>
          {icon}
        </div>
        <div>
          <p className="text-xs text-muted-foreground">{label}</p>
          <p className="text-lg font-semibold text-foreground">{value}</p>
        </div>
      </div>
    </Card>
  );
}

export function DashboardStats() {
  const { activeOrganization } = useOrganizationStore();
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!activeOrganization?.id) return;

    const loadStats = async () => {
      setLoading(true);
      try {
        // Fetch accounts for this org to calculate stats
        const data = await apiClient.listAccounts(activeOrganization.id);
        setAccounts(data);
      } catch (e) {
        console.error("Failed to fetch org stats", e);
      } finally {
        setLoading(false);
      }
    };
    loadStats();
  }, [activeOrganization?.id]);

  if (!activeOrganization) return null;

  const connectedCount = accounts.filter(
    (a) => a.status === "connected",
  ).length;
  const totalAccounts = accounts.length;

  // Placeholder for message count if we had an aggregate API
  // const messageCount = "0";

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
      <StatCard
        label="Organization"
        value={activeOrganization.name}
        icon={<Building2 className="w-4 h-4 text-[#25D366]" />}
        colorFn={() => "bg-[#25D366] text-[#25D366]"} // Whatsapp Green
      />
      <StatCard
        label="WhatsApp Accounts"
        value={`${connectedCount} / ${totalAccounts}`}
        icon={<Smartphone className="w-4 h-4 text-blue-500" />}
        colorFn={() => "bg-blue-500 text-blue-500"}
      />
      {/* 
            <StatCard 
               label="Messages (Today)" 
               value={messageCount}
               icon={<MessageSquare className="w-4 h-4 text-purple-500" />}
               colorFn={() => "bg-purple-500 text-purple-500"} 
            />
            */}
      <StatCard
        label="Integration Mode"
        value={activeOrganization.webhookUrl ? "Webhook Active" : "No Webhook"}
        icon={<Zap className="w-4 h-4 text-yellow-500" />}
        colorFn={() =>
          activeOrganization.webhookUrl
            ? "bg-yellow-500 text-yellow-500"
            : "bg-muted text-muted-foreground"
        }
      />
    </div>
  );
}
