"use client";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { authClient } from "@/lib/auth-client";
import { useOrganizationStore } from "@/stores/store";
import { Building2, Loader2 } from "lucide-react";
import { useRouter } from "next/navigation";
import { useState } from "react";

export default function CreateOrganizationPage() {
  const router = useRouter();
  const { setActiveOrganization } = useOrganizationStore();
  const [name, setName] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const handleCreate = async () => {
    if (!name.trim()) return;
    setLoading(true);
    setError("");

    try {
      const slug = name
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, "-")
        .replace(/^-+|-+$/g, "");
      const { data, error } = await authClient.organization.create({
        name,
        slug,
      });

      if (error) {
        setError(error.message || "Failed to create organization");
        return;
      }

      if (data) {
        // Set as active and redirect
        // We need to cast or map properly if types differ slightly between auth-client and our store
        setActiveOrganization({
          id: data.id,
          name: data.name,
          slug: data.slug,
          logo: data.logo || null,
          createdAt: data.createdAt,
        });
        router.push("/");
      }
    } catch (e: any) {
      console.error("Failed to create org", e);
      setError(e.message || "An unexpected error occurred");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <Card className="w-full max-w-md">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Building2 className="w-5 h-5 text-[#25D366]" />
            Create New Organization
          </CardTitle>
          <CardDescription>
            Create a workspace for your team or project.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="org-name">Organization Name</Label>
            <Input
              id="org-name"
              placeholder="My Awesome Company"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="bg-muted border-border text-foreground focus:ring-[#25D366]"
            />
          </div>

          {error && (
            <div className="text-sm p-3 rounded-md bg-destructive/10 text-destructive">
              {error}
            </div>
          )}
        </CardContent>
        <CardFooter className="flex justify-between">
          <Button
            variant="ghost"
            onClick={() => router.back()}
            className="text-muted-foreground hover:text-foreground"
          >
            Cancel
          </Button>
          <Button
            onClick={handleCreate}
            disabled={loading || !name.trim()}
            className="bg-[#25D366] hover:bg-[#20bd5a] text-black"
          >
            {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            Create Workspace
          </Button>
        </CardFooter>
      </Card>
    </div>
  );
}
