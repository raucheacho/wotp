"use client";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Check, Zap } from "lucide-react";

export default function BillingPage() {
  return (
    <div className="flex flex-col gap-6">
      <div>
          <h1 className="text-2xl font-bold text-foreground">Facturation & Abonnement</h1>
          <p className="text-muted-foreground">Gérez votre abonnement et vos factures.</p>
      </div>

      {/* Current Plan */}
      <Card className="relative overflow-hidden">
          <div className="absolute top-0 right-0 p-4 opacity-10">
              <Zap className="w-32 h-32 text-[#25D366]" />
          </div>
          <CardHeader>
              <div className="flex justify-between items-start">
                  <div>
                      <CardTitle className="text-xl">Plan Actuel</CardTitle>
                      <CardDescription>Vous êtes sur le plan gratuit.</CardDescription>
                  </div>
                  <Badge variant="outline" className="border-[#25D366] text-[#25D366] px-3 py-1">Gratuit</Badge>
              </div>
          </CardHeader>
          <CardContent>
              <div className="flex items-end gap-2 mb-4">
                  <span className="text-4xl font-bold text-foreground">0€</span>
                  <span className="text-muted-foreground mb-1">/mois</span>
              </div>
              <div className="w-full bg-muted rounded-full h-2 mb-2">
                  <div className="bg-[#25D366] h-2 rounded-full w-[20%]" />
              </div>
              <p className="text-xs text-muted-foreground">1 / 5 comptes connectés</p>
          </CardContent>
      </Card>    

      <h2 className="text-xl font-bold text-foreground mt-4">Plans Disponibles</h2>
      
      <div className="grid md:grid-cols-3 gap-6">
          {/* START */}
          <Card className="flex flex-col">
              <CardHeader>
                  <CardTitle>Starter</CardTitle>
                  <CardDescription>Pour démarrer avec WhatsApp.</CardDescription>
              </CardHeader>
              <CardContent className="flex-1">
                  <div className="text-3xl font-bold text-foreground mb-4">29€<span className="text-sm font-normal text-muted-foreground">/mo</span></div>
                  <ul className="space-y-2 text-sm text-muted-foreground">
                      <li className="flex items-center gap-2"><Check className="w-4 h-4 text-[#25D366]" /> 5 Comptes WhatsApp</li>
                      <li className="flex items-center gap-2"><Check className="w-4 h-4 text-[#25D366]" /> Webhooks basiques</li>
                      <li className="flex items-center gap-2"><Check className="w-4 h-4 text-[#25D366]" /> Support Email</li>
                  </ul>
              </CardContent>
              <CardFooter>
                  <Button variant="outline" className="w-full border-border text-muted-foreground hover:bg-muted">Passer au Starter</Button>
              </CardFooter>
          </Card>

          {/* PRO */}
          <Card className="bg-[#25D366]/10 border-[#25D366]/50 flex flex-col relative">
              <div className="absolute -top-3 left-1/2 -translate-x-1/2 bg-[#25D366] text-black text-xs font-bold px-3 py-1 rounded-full">Recommandé</div>
              <CardHeader>
                  <CardTitle className="text-[#25D366]">Pro</CardTitle>
                  <CardDescription className="text-zinc-400">Pour les entreprises en croissance.</CardDescription>
              </CardHeader>
              <CardContent className="flex-1">
                  <div className="text-3xl font-bold text-white mb-4">99€<span className="text-sm font-normal text-zinc-500">/mo</span></div>
                  <ul className="space-y-2 text-sm text-zinc-300">
                      <li className="flex items-center gap-2"><Check className="w-4 h-4 text-[#25D366]" /> 20 Comptes WhatsApp</li>
                      <li className="flex items-center gap-2"><Check className="w-4 h-4 text-[#25D366]" /> Webhooks avancés</li>
                       <li className="flex items-center gap-2"><Check className="w-4 h-4 text-[#25D366]" /> Support Prioritaire</li>
                       <li className="flex items-center gap-2"><Check className="w-4 h-4 text-[#25D366]" /> API Illimitée</li>
                  </ul>
              </CardContent>
              <CardFooter>
                  <Button className="w-full bg-[#25D366] text-white hover:bg-[#1ebe5d]">Choisir Pro</Button>
              </CardFooter>
          </Card>

          {/* ENTERPRISE */}
          <Card className="flex flex-col">
              <CardHeader>
                  <CardTitle>Enterprise</CardTitle>
                  <CardDescription>Solutions sur mesure.</CardDescription>
              </CardHeader>
              <CardContent className="flex-1">
                  <div className="text-3xl font-bold text-foreground mb-4">Sur devis</div>
                  <ul className="space-y-2 text-sm text-muted-foreground">
                      <li className="flex items-center gap-2"><Check className="w-4 h-4 text-[#25D366]" /> Comptes Illimités</li>
                      <li className="flex items-center gap-2"><Check className="w-4 h-4 text-[#25D366]" /> Infrastructure Dédiée</li>
                      <li className="flex items-center gap-2"><Check className="w-4 h-4 text-[#25D366]" /> SLA Garanti</li>
                  </ul>
              </CardContent>
              <CardFooter>
                  <Button variant="outline" className="w-full border-border text-muted-foreground hover:bg-muted">Contacter nous</Button>
              </CardFooter>
          </Card>
      </div>

    </div>
  );
}
