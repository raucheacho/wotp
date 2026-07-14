"use client";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { type Account, apiClient } from "@/lib/api-client";
import { AlertCircle, Check, Loader2, Send } from "lucide-react";
import { useState } from "react";

export function WebhookTest({ account }: { account: Account }) {
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<{ success: boolean; message: string; details?: any } | null>(null);

  const handleTest = async () => {
    if (!account.webhookUrl) return;

    setLoading(true);
    setResult(null);
    try {
      const res = await apiClient.testWebhook(account.id);
      setResult(res);
    } catch (error: any) {
      setResult({
        success: false,
        message: error.message || "Erreur lors du test du webhook",
        details: error
      });
    } finally {
      setLoading(false);
    }
  };

  if (!account.webhookUrl) {
    return (
      <Card className="opacity-60">
        <CardHeader>
          <CardTitle className="text-lg font-medium">Test du Webhook</CardTitle>
          <CardDescription>
            Configurez une URL de webhook pour activer cette fonctionnalité.
          </CardDescription>
        </CardHeader>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg font-medium">Test du Webhook</CardTitle>
        <CardDescription>
          Envoyez un événement de test vers <code>{account.webhookUrl}</code> pour vérifier la configuration.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <Button 
          onClick={handleTest} 
          disabled={loading}
          className="bg-[#25D366] hover:bg-[#1ebe5d] text-white"
        >
          {loading ? (
            <Loader2 className="h-4 w-4 mr-2 animate-spin" />
          ) : (
            <Send className="h-4 w-4 mr-2" />
          )}
          Envoyer un événement de test
        </Button>

        {result && (
          <div className={`mt-4 p-4 rounded-lg flex items-start gap-3 border ${
            result.success 
              ? "border-green-500/50 bg-green-500/10 text-green-700 dark:text-green-400" 
              : "border-destructive/50 bg-destructive/10 text-destructive"
          }`}>
            {result.success ? <Check className="h-5 w-5 mt-0.5 shrink-0" /> : <AlertCircle className="h-5 w-5 mt-0.5 shrink-0" />}
            <div>
              <h5 className="font-medium mb-1">{result.success ? "Succès" : "Échec"}</h5>
              <p className="text-sm opacity-90">{result.message}</p>
              {/* {result.details && (
                <pre className="mt-2 p-2 bg-black/40 rounded overflow-auto max-h-40 text-xs">
                  {JSON.stringify(result.details, null, 2)}
                </pre>
              )} */}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
