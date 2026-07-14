"use client";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { authClient } from "@/lib/auth-client";
import { Loader2 } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useState } from "react";

function ResetPasswordContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  // Better-Auth typically puts the token/error in URL. 
  // Normally reset logic is token-based. authClient.resetPassword handles it if we have 'token' query param? 
  // Actually, better-auth flow redirects with a token. We need to grab it?
  // Check docs: usually "token" or "code".
  
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [success, setSuccess] = useState(false);

  const handleResetPassword = async () => {
    if (password !== confirmPassword) {
      alert("Les mots de passe ne correspondent pas");
      return;
    }
    setLoading(true);
    await (authClient as any).resetPassword({
      newPassword: password,
      // The token is automatically extracted from URL by better-auth client if present? 
      // Or we pass it explicitly? The client usually handles it if it's in the URL, 
      // OR we need to pass `token` param. Let's assume automatic or we pass it?
      // Better-auth client `resetPassword` signature: ({ newPassword, token?, ... })
      // If token is in search params 'token', we can pass it to be safe.
      token: searchParams.get("token") || undefined 
    }, {
      onSuccess: () => {
        setSuccess(true);
        setLoading(false);
        setTimeout(() => {
          router.push("/auth/sign-in");
        }, 2000);
      },
      onError: (ctx: any) => {
        alert(ctx.error.message);
        setLoading(false);
      }
    });
  };

  if (success) {
    return (
      <CardContent className="space-y-4">
        <div className="bg-emerald-500/10 text-emerald-500 p-4 rounded-lg text-center">
          Mot de passe réinitialisé avec succès !
        </div>
        <p className="text-center text-zinc-400">
          Redirection vers la connexion...
        </p>
      </CardContent>
    );
  }

  return (
    <CardContent className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="password">Nouveau mot de passe</Label>
        <Input 
          id="password" 
          type="password" 
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          className="bg-muted border-border text-foreground"
        />
      </div>
      <div className="space-y-2">
        <Label htmlFor="confirmPassword">Confirmer le mot de passe</Label>
        <Input 
          id="confirmPassword" 
          type="password" 
          value={confirmPassword}
          onChange={(e) => setConfirmPassword(e.target.value)}
          className="bg-muted border-border text-foreground"
        />
      </div>
      <Button 
        className="w-full bg-primary text-primary-foreground hover:bg-primary/90" 
        onClick={handleResetPassword}
        disabled={loading}
      >
        {loading ? <Loader2 className="animate-spin w-4 h-4 mr-2" /> : null}
        Réinitialiser
      </Button>
    </CardContent>
  );
}

export default function ResetPasswordPage() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <Card className="w-full max-w-md">
        <CardHeader>
          <CardTitle className="text-2xl font-bold text-center">Nouveau mot de passe</CardTitle>
          <CardDescription className="text-center text-muted-foreground">
            Choisissez un nouveau mot de passe sécurisé
          </CardDescription>
        </CardHeader>
        <Suspense fallback={<CardContent>Chargement...</CardContent>}>
          <ResetPasswordContent />
        </Suspense>
      </Card>
    </div>
  );
}
