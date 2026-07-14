"use client";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { authClient } from "@/lib/auth-client";
import { Loader2 } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";

export default function SignUpPage() {
  const router = useRouter();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [orgName, setOrgName] = useState("");
  const [loading, setLoading] = useState(false);

  const handleSignUp = async () => {
    setLoading(true);
    // 1. Sign Up User
    await authClient.signUp.email({
      email,
      password,
      name,
    }, {
      onSuccess: async () => {
         // 2. Create Organization
         // Sign Up automatically logs in, but we need to await the session token to be available
         try {
             if (orgName) {
                // Explicitly sign in again just to be sure we have a fresh session if needed, 
                // though usually signUp behaves as signIn. 
                // A better approach is to wrap in try-catch and maybe wait a tick.
                
                await authClient.organization.create({
                    name: orgName,
                    slug: orgName.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, ''),
                });
             }
             router.push("/");
         } catch (e: any) {
             console.error("Failed to create org:", e);
             // Even if org creation fails, user is created. 
             // Maybe redirect to an onboarding "Create Org" page instead?
             // For now, let's just alert and push to home.
             alert("Compte créé mais erreur lors de la création de l'organisation: " + e.message);
             router.push("/");
         }
      },
      onError: (ctx) => {
        alert(ctx.error.message);
        setLoading(false);
      }
    });
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <Card className="w-full max-w-md">
        <CardHeader>
          <CardTitle className="text-2xl font-bold text-center">Inscription</CardTitle>
          <CardDescription className="text-center text-muted-foreground">
            Créez votre compte et votre organisation
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="name">Nom complet</Label>
            <Input 
              id="name" 
              placeholder="John Doe"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="bg-muted border-border text-foreground"
            />
          </div>
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
          <div className="space-y-2">
            <Label htmlFor="org">Nom de l'organisation</Label>
            <Input 
              id="org" 
              placeholder="Ma Société"
              value={orgName}
              onChange={(e) => setOrgName(e.target.value)}
              className="bg-muted border-border text-foreground"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="password">Mot de passe</Label>
            <Input 
              id="password" 
              type="password" 
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="bg-muted border-border text-foreground"
            />
          </div>
          <Button 
            className="w-full bg-primary text-primary-foreground hover:bg-primary/90" 
            onClick={handleSignUp}
            disabled={loading}
          >
            {loading ? <Loader2 className="animate-spin w-4 h-4 mr-2" /> : null}
            Commencer
          </Button>
        </CardContent>
        <CardFooter className="justify-center">
          <p className="text-sm text-muted-foreground">
            Déjà un compte ?{" "}
            <Link href="/auth/sign-in" className="text-foreground hover:underline">
              Se connecter
            </Link>
          </p>
        </CardFooter>
      </Card>
    </div>
  );
}
