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
import { authClient } from "@/lib/auth-client";
import { Loader2, LoaderCircle, Save } from "lucide-react";
import { useState } from "react";
// import { toast } from "sonner"; // Fallback to alert

export function GeneralSettingsTab() {
  const { data: org, isPending, refetch } = authClient.useActiveOrganization();

  const [name, setName] = useState(() => org?.name ?? "");
  const [slug, setSlug] = useState(() => org?.slug ?? "");

  const [saving, setSaving] = useState(false);

  console.log("org: ", org);

  const handleSave = async () => {
    setSaving(true);
    try {
      await authClient.organization.update(
        {
          data: {
            name,
            slug: slug || undefined,
          },
        },
        {
          onSuccess: () => {
            alert("Organisation mise à jour !");
            refetch();
          },
          onError: (ctx) => {
            alert(ctx.error.message);
          },
        },
      );
    } catch (e) {
      console.error(e);
    }
    setSaving(false);
  };

  if (isPending)
    return (
      <div className="p-4 w-full flex items-center justify-center text-muted-foreground">
        <LoaderCircle className="animate-spin" />{" "}
        <span className="ml-1">Chargement...</span>
      </div>
    );

  return (
    <Card className="mt-4">
      <CardHeader>
        <CardTitle>Informations Générales</CardTitle>
        <CardDescription>
          {"Modifiez le nom et l'identifiant de votre organisation."}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2">
          <Label className="text-muted-foreground">
            {"Nom de l'organisation"}
          </Label>
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="bg-muted border-border text-foreground"
          />
        </div>

        <div className="space-y-2">
          <Label className="text-muted-foreground">
            Slug (Identifiant unique URL)
          </Label>
          <Input
            value={slug}
            onChange={(e) => setSlug(e.target.value)}
            className="bg-muted border-border text-foreground"
          />
          <p className="text-xs text-muted-foreground">
            {"Utilisé pour l'URL de votre organisation."}
          </p>
        </div>

        <div className="pt-2 flex justify-end">
          <Button
            onClick={handleSave}
            disabled={saving}
            className="bg-[#25D366] text-white hover:bg-[#1ebe5d]"
          >
            {saving ? (
              <Loader2 className="w-4 h-4 animate-spin mr-2" />
            ) : (
              <Save className="w-4 h-4 mr-2" />
            )}
            Sauvegarder
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
