"use client";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { type Account, apiClient } from "@/lib/api-client";
import { Check, Copy, RefreshCw } from "lucide-react";
import { useState } from "react";
// If Alert component doesn't exist, I'll fallback to divs. Based on file list, it might not exist.

export function AccountApiKeys({ account, onUpdate }: { account: Account; onUpdate: (account: Account) => void }) {
  const [loading, setLoading] = useState(false);
  const [copiedId, setCopiedId] = useState(false);
  const [copiedToken, setCopiedToken] = useState(false);

  const copyToClipboard = async (text: string, type: "id" | "token") => {
    await navigator.clipboard.writeText(text);
    if (type === "id") {
      setCopiedId(true);
      setTimeout(() => setCopiedId(false), 2000);
    } else {
      setCopiedToken(true);
      setTimeout(() => setCopiedToken(false), 2000);
    }
  };

  const handleRegenerate = async () => {
    if (!confirm("Attention: La régénération du token invalidera l'ancien token. Tous les scripts utilisant l'ancien token devront être mis à jour. Continuer ?")) {
      return;
    }

    setLoading(true);
    try {
      const updated = await apiClient.regenerateAccountToken(account.id);
      onUpdate(updated);
    } catch (error) {
      console.error("Failed to regenerate token:", error);
      alert("Erreur lors de la régénération du token");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg font-medium">Identifiants API</CardTitle>
        <CardDescription>
          Ces identifiants permettent de connecter vos applications à ce compte WhatsApp.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2">
          <Label>Account ID</Label>
          <div className="flex gap-2">
            <Input 
              value={account.id} 
              readOnly 
              className="font-mono bg-muted"
            />
            <Button
              variant="outline"
              size="icon"
              className="hover:bg-muted"
              onClick={() => copyToClipboard(account.id, "id")}
            >
              {copiedId ? <Check className="h-4 w-4 text-green-500" /> : <Copy className="h-4 w-4 text-muted-foreground" />}
            </Button>
          </div>
        </div>

        <div className="space-y-2">
          <Label>Public Token</Label>
          <div className="flex gap-2">
            <Input 
              value={account.publicToken} 
              readOnly 
              type="password"
              className="font-mono bg-muted"
            />
             <Button
              variant="outline"
              size="icon"
              className="hover:bg-muted"
              onClick={() => copyToClipboard(account.publicToken, "token")}
            >
              {copiedToken ? <Check className="h-4 w-4 text-green-500" /> : <Copy className="h-4 w-4 text-muted-foreground" />}
            </Button>
          </div>
          <p className="text-xs text-muted-foreground">
            Utilisé pour authentifier les requêtes WebSocket et API publique.
          </p>
        </div>

        <div className="pt-2 border-t border-border">
          <Button 
            variant="ghost" 
            className="text-destructive hover:text-destructive hover:bg-destructive/10 w-full justify-start pl-0"
            onClick={handleRegenerate}
            disabled={loading}
          >
            {loading ? (
              <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
            ) : (
              <RefreshCw className="h-4 w-4 mr-2" />
            )}
            Régénérer le token
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
