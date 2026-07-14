"use client";

import { Building2, Check, ChevronsUpDown, PlusCircle } from "lucide-react";
import * as React from "react";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";

import { authClient } from "@/lib/auth-client";
import { useParams, useRouter } from "next/navigation";

export function OrganizationSwitcher({
  className,
}: React.HTMLAttributes<HTMLDivElement>) {
  const router = useRouter();
  const params = useParams();
  const currentOrgId = params.orgId as string;
  
  const { data: organizations, isPending } = authClient.useListOrganizations();
  const { data: session } = authClient.useSession();

  console.log('[OrganizationSwitcher] organizations:', organizations);
  console.log('[OrganizationSwitcher] isPending:', isPending);
  console.log('[OrganizationSwitcher] currentOrgId:', currentOrgId);

  // Find current organization based on URL param
  const currentOrg = organizations?.find(org => org.id === currentOrgId) || organizations?.[0];

  // Handle organization switch
  const handleOrgSwitch = (orgId: string) => {
    router.push(`/${orgId}`);
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="outline"
          role="combobox"
          aria-label="Select an organization"
          className={cn("w-full justify-between h-12 px-3 border-border bg-card text-card-foreground hover:bg-accent hover:text-accent-foreground", className)}
        >
          <div className="flex items-center gap-3 text-left">
             <div className="flex items-center justify-center w-8 h-8 rounded-md bg-muted border border-border">
                {currentOrg?.logo ? (
                   <img src={currentOrg.logo} alt={currentOrg.name} className="w-full h-full object-cover rounded-md" />
                ) : (
                    <Building2 className="w-4 h-4 text-muted-foreground" />
                )}
             </div>
             <div className="flex flex-col overflow-hidden">
                <span className="truncate text-sm font-medium leading-tight">
                    {currentOrg?.name || "Select Organization"}
                </span>
                <span className="truncate text-xs text-muted-foreground">
                    {currentOrg?.slug || "Workspace"}
                </span>
             </div>
          </div>
          <ChevronsUpDown className="ml-auto h-4 w-4 shrink-0 opacity-50" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent className="w-[240px]">
        <DropdownMenuGroup>
          <DropdownMenuLabel className="px-2 py-1.5 text-xs font-semibold text-muted-foreground">
            Organizations
          </DropdownMenuLabel>
          {organizations?.map((org) => (
            <DropdownMenuItem
              key={org.id}
              onSelect={() => handleOrgSwitch(org.id)}
              className="text-sm cursor-pointer gap-2"
            > 
             <div className="flex items-center gap-2 flex-1">
                <Building2 className="w-4 h-4 text-muted-foreground" />
                <span>{org.name}</span>
             </div>
              {currentOrg?.id === org.id && (
                <Check className="ml-auto h-4 w-4 text-primary" />
              )}
            </DropdownMenuItem>
          ))}
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuItem 
            className="text-sm cursor-pointer gap-2"
            onSelect={() => router.push("/onboarding/create-organization")}
        >
          <PlusCircle className="h-4 w-4" />
          Create Organization
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
