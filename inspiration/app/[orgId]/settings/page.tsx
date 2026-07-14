import { GeneralSettingsTab } from "@/components/settings/GeneralSettingsTab";
import { MembersTab } from "@/components/settings/MembersTab";
import { OrganizationDetailsCard } from "@/components/settings/OrganizationDetailsCard";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { apiClient } from "@/lib/api-client";

export default async function SettingsPage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = await params;
  const data = await apiClient.getOrganization(orgId);

  return (
    <>
      <h1 className="text-2xl font-bold text-foreground mb-6">Paramètres</h1>

      <Tabs defaultValue="organization" className="w-full">
        <TabsList className="bg-muted border-border text-muted-foreground">
          <TabsTrigger value="organization">Organisation</TabsTrigger>
          <TabsTrigger value="members">Membres & Équipe</TabsTrigger>
        </TabsList>

        <TabsContent value="organization">
          <OrganizationDetailsCard organization={data} />
          <GeneralSettingsTab />
        </TabsContent>

        <TabsContent value="members">
          <MembersTab />
        </TabsContent>
      </Tabs>
    </>
  );
}
