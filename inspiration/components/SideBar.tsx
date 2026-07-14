"use client";
import { authClient } from "@/lib/auth-client";
import { useOrgUrl } from "@/lib/url-helpers";
import { cn } from "@/lib/utils";
import { Building2, Code, CreditCard, ScrollText, Settings, ShieldAlert, Smartphone, Webhook } from "lucide-react";
import Link from "next/link";
import { useParams, usePathname } from "next/navigation";
import { OrganizationSwitcher } from "./OrganizationSwitcher";

export function SideBar() {
  const pathname = usePathname();
  const params = useParams();
  const getUrl = useOrgUrl();
  const { data: session } = authClient.useSession();
  const user = session?.user;

  const orgId = params.orgId as string;

  // Check if we're in an org context
  if (!orgId) {
    return null;
  }

  // Helper to check if current path matches (ignoring org prefix)
  const isActive = (path: string) => {
    const fullPath = `/${orgId}${path}`;
    if (path === "/") {
      // Remove trailing slash for comparison if present
      const normalizedPathname = pathname.endsWith('/') && pathname !== '/' ? pathname.slice(0, -1) : pathname;
      const normalizedFullPath = fullPath.endsWith('/') && fullPath !== '/' ? fullPath.slice(0, -1) : fullPath;
      return normalizedPathname === normalizedFullPath;
    }
    return pathname.startsWith(fullPath);
  };

  return (
    <aside className="w-1/4 p-3 overflow-y-auto">
      <div className="border-r border-sidebar-border h-full flex flex-col">
        <nav className="p-2 flex-1">
          <div className="space-y-1">
            <div className="px-1 py-2 mb-4">
              <OrganizationSwitcher />
            </div>

            <Link
              href={getUrl("/")}
              className={cn(
                "flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors",
                isActive("/")
                  ? "bg-[#25D366]/10 text-[#25D366]"
                  : "text-muted-foreground hover:text-sidebar-foreground hover:bg-sidebar-accent"
              )}
            >
              <Building2 className="w-4 h-4" />
              Vue d'ensemble
            </Link>
            
            <p className="px-3 pt-4 pb-2 text-[10px] font-semibold text-muted-foreground uppercase tracking-wider">
              Gestion
            </p>

            <Link
              href={getUrl("/accounts")}
              className={cn(
                "flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors",
                isActive("/accounts")
                  ? "bg-[#25D366]/10 text-[#25D366]"
                  : "text-muted-foreground hover:text-sidebar-foreground hover:bg-sidebar-accent"
              )}
            >
              <Smartphone className="w-4 h-4" />
              Comptes
            </Link>

            <Link
              href={getUrl("/webhooks")}
              className={cn(
                "flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors",
                isActive("/webhooks")
                  ? "bg-[#25D366]/10 text-[#25D366]"
                  : "text-muted-foreground hover:text-sidebar-foreground hover:bg-sidebar-accent"
              )}
            >
              <Webhook className="w-4 h-4" />
              Webhooks
            </Link>

            <p className="px-3 pt-4 pb-2 text-[10px] font-semibold text-muted-foreground uppercase tracking-wider">
              Configuration
            </p>

            <Link
              href={getUrl("/settings")}
              className={cn(
                "flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors",
                isActive("/settings")
                  ? "bg-[#25D366]/10 text-[#25D366]"
                  : "text-muted-foreground hover:text-sidebar-foreground hover:bg-sidebar-accent"
              )}
            >
              <Settings className="w-4 h-4" />
              Paramètres
            </Link>

            <Link
              href={getUrl("/billing")}
              className={cn(
                "flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors",
                isActive("/billing")
                  ? "bg-[#25D366]/10 text-[#25D366]"
                  : "text-muted-foreground hover:text-sidebar-foreground hover:bg-sidebar-accent"
              )}
            >
              <CreditCard className="w-4 h-4" />
              Facturation
            </Link>

             <Link
              href={getUrl("/api-docs")}
              className={cn(
                "flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors",
                isActive("/api-docs")
                  ? "bg-[#25D366]/10 text-[#25D366]"
                  : "text-muted-foreground hover:text-sidebar-foreground hover:bg-sidebar-accent"
              )}
            >
              <Code className="w-4 h-4" />
              API & Webhooks
            </Link>

            <p className="px-3 pt-4 pb-2 text-[10px] font-semibold text-muted-foreground uppercase tracking-wider">
              Monitoring
            </p>

             <Link
              href={getUrl("/logs")}
              className={cn(
                "flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors",
                isActive("/logs")
                  ? "bg-[#25D366]/10 text-[#25D366]"
                  : "text-muted-foreground hover:text-sidebar-foreground hover:bg-sidebar-accent"
              )}
            >
              <ScrollText className="w-4 h-4" />
              Logs
            </Link>


            {/* Admin Link (Only for Super Admin) */}
            {user?.role === "admin" && (
                <>
                <div className="h-px bg-sidebar-border my-2" />
                <Link
                href="/admin"
                className={cn(
                    "flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors",
                    pathname === "/admin"
                    ? "bg-destructive/10 text-destructive"
                    : "text-muted-foreground hover:text-sidebar-foreground hover:bg-sidebar-accent"
                )}
                >
                <ShieldAlert className="w-4 h-4" />
                Administration
                </Link>
                </>
            )}
          </div>
        </nav>
      </div>
    </aside>
  );
}


export default SideBar;
