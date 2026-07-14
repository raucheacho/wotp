"use client";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ArrowLeft, Loader2 } from "lucide-react";
import Link from "next/link";
import { useState } from "react";

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState("");
  const [loading, setLoading] = useState(false);
  const [submitted, setSubmitted] = useState(false);

  const handleReset = async () => {
    if (!email) return;
    setLoading(true);
    try {
      const res = await fetch("/api/services/auth/forgot-password", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          email,
          redirectTo: "/auth/reset-password"
        })
      });

      const data = await res.json();
      
      if (!res.ok) {
        throw new Error(data.error || "Une erreur est survenue");
      }

      setSubmitted(true);
      setLoading(false);
    } catch (error: any) {
      alert(error.message);
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <Card className="w-full max-w-md">
        <CardHeader>
          <CardTitle className="text-2xl font-bold text-center">Mot de passe oublié</CardTitle>
          <CardDescription className="text-center text-muted-foreground">
            Entrez votre email pour recevoir un lien de réinitialisation
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {!submitted ? (
            <div className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="email">Email</Label>
                <Input 
                  id="email" 
                  type="email" 
                  placeholder="votre@email.com"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="bg-muted border-border text-foreground"
                />
              </div>
              <Button 
                className="w-full bg-primary text-primary-foreground hover:bg-primary/90" 
                onClick={handleReset}
                disabled={loading}
              >
                {loading ? <Loader2 className="animate-spin w-4 h-4 mr-2" /> : null}
                Envoyer le lien
              </Button>
            </div>
          ) : (
            <div className="text-center space-y-4">
              <div className="bg-emerald-500/10 text-emerald-500 p-4 rounded-lg">
                Email envoyé ! Vérifiez votre boîte de réception (et vos spams).
              </div>
              <p className="text-muted-foreground text-sm">
                Si vous ne recevez rien, vérifiez que l'adresse est correcte.
              </p>
            </div>
          )}
        </CardContent>
        <CardFooter className="justify-center">
          <Link href="/auth/sign-in" className="flex items-center text-sm text-muted-foreground hover:text-foreground">
            <ArrowLeft className="w-4 h-4 mr-2" />
            Retour à la connexion
          </Link>
        </CardFooter>
      </Card>
    </div>
  );
}
