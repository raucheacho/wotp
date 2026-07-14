"use client";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { apiClient } from "@/lib/api-client";
import { useOrganizationStore } from "@/stores/store";
import { Loader2, Save, Webhook } from "lucide-react";
import { useEffect, useState } from "react";
import { z } from "zod";

export function WebhookSettingsCard() {
  const { activeOrganization } = useOrganizationStore();
  const [webhookUrl, setWebhookUrl] = useState("");
  const [webhookSecret, setWebhookSecret] = useState("");
  const [loading, setLoading] = useState(false);
  const [fetching, setFetching] = useState(false);

  // Simple state for feedback since useToast is not available
  const [message, setMessage] = useState<{
    type: "success" | "error";
    text: string;
  } | null>(null);

  useEffect(() => {
    if (activeOrganization?.id) {
      fetchWebhookSettings(activeOrganization.id);
    }
  }, [activeOrganization?.id]);

  const fetchWebhookSettings = async (orgId: string) => {
    setFetching(true);
    try {
      const org = await apiClient.getOrganization(orgId);
      if (org) {
        setWebhookUrl(org.webhookUrl || "");
        setWebhookSecret(org.webhookSecret || "");
      }
    } catch (error) {
      console.error("Failed to fetch webhook settings", error);
    } finally {
      setFetching(false);
    }
  };

  const handleSave = async () => {
    if (!activeOrganization?.id) return;

    setLoading(true);
    setMessage(null);

    try {
      // Basic validation
      if (webhookUrl && !z.string().url().safeParse(webhookUrl).success) {
        throw new Error("Invalid URL format");
      }

      await apiClient.updateOrganization(activeOrganization.id, {
        webhookUrl: webhookUrl || null,
        webhookSecret,
      });
      setMessage({
        type: "success",
        text: "Webhook settings updated successfully",
      });
    } catch (error: any) {
      console.error("Failed to update webhook", error);
      setMessage({
        type: "error",
        text: error.message || "Failed to update settings",
      });
    } finally {
      setLoading(false);
    }
  };

  if (!activeOrganization) {
    return (
      <Card className="bg-card border-border text-card-foreground">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Webhook className="w-5 h-5" />
            Webhooks configuration
          </CardTitle>
          <CardDescription>
            Please select an organization from the sidebar to configure
            webhooks.
          </CardDescription>
        </CardHeader>
      </Card>
    );
  }

  return (
    <Card className="bg-card border-border text-card-foreground">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Webhook className="w-5 h-5 text-[#25D366]" />
          Webhook Configuration
        </CardTitle>
        <CardDescription className="text-muted-foreground">
          Configure the endpoint for {activeOrganization.name} to receive
          real-time events.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {fetching ? (
          <div className="flex justify-center p-4">
            <Loader2 className="animate-spin w-6 h-6 text-muted-foreground" />
          </div>
        ) : (
          <>
            <div className="space-y-2">
              <Label htmlFor="webhook-url">Webhook URL</Label>
              <Input
                id="webhook-url"
                placeholder="https://api.yourdomain.com/webhook"
                value={webhookUrl}
                onChange={(e) => setWebhookUrl(e.target.value)}
                className="bg-muted border-input text-foreground focus:ring-[#25D366]"
              />
              <p className="text-xs text-muted-foreground">
                HTTPS is recommended for security.
              </p>
            </div>

            <div className="space-y-2">
              <Label htmlFor="webhook-secret">Signing Secret</Label>
              <div className="flex gap-2">
                <Input
                  id="webhook-secret"
                  type="password"
                  placeholder="whsec_..."
                  value={webhookSecret}
                  onChange={(e) => setWebhookSecret(e.target.value)}
                  className="bg-muted border-input text-foreground font-mono"
                />
                <Button
                  variant="outline"
                  onClick={() =>
                    setWebhookSecret(
                      "whsec_" + crypto.randomUUID().replace(/-/g, ""),
                    )
                  }
                  className="border-input hover:bg-muted text-foreground"
                >
                  Generate
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">
                Used to verify the authenticity of payloads.
              </p>
            </div>

            {message && (
              <div
                className={`text-sm p-3 rounded-md ${message.type === "success" ? "bg-green-500/10 text-green-500" : "bg-destructive/10 text-destructive"}`}
              >
                {message.text}
              </div>
            )}

            <div className="pt-2">
              <Button
                onClick={handleSave}
                disabled={loading}
                className="bg-[#25D366] hover:bg-[#20bd5a] text-black w-full sm:w-auto"
              >
                {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                <Save className="mr-2 h-4 w-4" />
                Save Changes
              </Button>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}
