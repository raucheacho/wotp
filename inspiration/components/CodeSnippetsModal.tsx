"use client";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Account } from "@/lib/api-client";
import { Check, Code2, Copy } from "lucide-react";
import { useState } from "react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8000";

export function CodeSnippetsModal({
  open,
  onClose,
  account,
}: {
  open: boolean;
  onClose: () => void;
  account: Account;
}) {
  const [copied, setCopied] = useState<string | null>(null);

  const copyToClipboard = async (text: string, id: string) => {
    await navigator.clipboard.writeText(text);
    setCopied(id);
    setTimeout(() => setCopied(null), 2000);
  };

  const curlExample = `curl -X POST ${API_URL}/v1/accounts/${account.id}/messages/text \\
  -H "X-API-Key: YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "to": "1234567890",
    "text": "Hello world from API!"
  }'`;

  const jsExample = `const response = await fetch('${API_URL}/v1/accounts/${account.id}/messages/text', {
  method: 'POST',
  headers: {
    'X-API-Key': 'YOUR_API_KEY',
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    to: '1234567890',
    text: 'Hello world from API!'
  })
});

const data = await response.json();
console.log(data);`;

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Code2 className="w-5 h-5" /> Exemples de Code
          </DialogTitle>
          <DialogDescription>
            Utilisez ces extraits pour intégrer l'API WhatsApp dans votre application.
          </DialogDescription>
        </DialogHeader>

        <div className="mt-4">
          <Tabs defaultValue="curl" className="w-full">
            <TabsList className="bg-muted border border-border">
              <TabsTrigger value="curl">cURL</TabsTrigger>
              <TabsTrigger value="js">Node.js / JS</TabsTrigger>
            </TabsList>

            <TabsContent value="curl" className="mt-4 space-y-2">
              <div className="flex justify-between items-center">
                <p className="text-sm font-medium text-muted-foreground">Envoyer un message</p>
                <Button variant="ghost" size="sm" onClick={() => copyToClipboard(curlExample, "curl")}>
                  {copied === "curl" ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
                </Button>
              </div>
              <pre className="p-4 rounded-lg bg-muted border border-border text-xs font-mono overflow-x-auto text-foreground">
                {curlExample}
              </pre>
            </TabsContent>

            <TabsContent value="js" className="mt-4 space-y-2">
               <div className="flex justify-between items-center">
                <p className="text-sm font-medium text-muted-foreground">Envoyer un message</p>
                <Button variant="ghost" size="sm" onClick={() => copyToClipboard(jsExample, "js")}>
                   {copied === "js" ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
                </Button>
              </div>
              <pre className="p-4 rounded-lg bg-muted border border-border text-xs font-mono overflow-x-auto text-blue-400">
                {jsExample}
              </pre>
            </TabsContent>
          </Tabs>
        </div>
      </DialogContent>
    </Dialog>
  );
}
