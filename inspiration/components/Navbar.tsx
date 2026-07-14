"use client";
import { MessageSquare } from "lucide-react";
import { ThemeToggle } from "./theme/theme-toggle";

export function Navbar() {
  return (
    <nav className="fixed top-0 left-0 right-0 h-14 bg-background border-b border-border z-50">
      <div className="h-full px-4 flex items-center justify-between max-w-6xl mx-auto">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-lg bg-[#25D366] flex items-center justify-center">
            <MessageSquare className="w-5 h-5 text-white" />
          </div>
          <span className="font-semibold text-foreground">
            Chat Gate
          </span>
        </div>

        <div className="flex items-center gap-3">
          <ThemeToggle />
          <div className="flex items-center gap-4">
            {/* User Menu */}
            <UserMenu />
          </div>
        </div>
      </div>
    </nav>
  );
}

import { authClient } from "@/lib/auth-client";
import { LogOut } from "lucide-react";
import { useRouter } from "next/navigation";
import { useState } from "react";

function UserMenu() {
  const { data: session } = authClient.useSession();
  const router = useRouter();
  const [open, setOpen] = useState(false);

  if (!session) return null;

  return (
    <div className="relative">
      <button
        onClick={() => setOpen(!open)}
        className="flex items-center gap-2 text-muted-foreground hover:text-foreground transition-colors"
      >
        <div className="w-8 h-8 rounded-full bg-muted flex items-center justify-center border border-border">
          <span className="font-medium text-sm text-foreground">
            {session.user.name?.charAt(0).toUpperCase()}
          </span>
        </div>
      </button>

      {open && (
        <div className="absolute right-0 top-10 w-48 bg-card border border-border rounded-lg shadow-xl py-1 z-50">
          <div className="px-3 py-2 border-b border-border mb-1">
            <p className="text-sm font-medium text-foreground truncate">
              {session.user.name}
            </p>
            <p className="text-xs text-muted-foreground truncate">
              {session.user.email}
            </p>
          </div>
          <button
            onClick={async () => {
              await authClient.signOut();
              router.push("/auth/sign-in");
            }}
            className="w-full text-left px-3 py-2 text-sm text-destructive hover:bg-muted flex items-center gap-2"
          >
            <LogOut className="w-4 h-4" />
            Se déconnecter
          </button>
        </div>
      )}

      {/* Backdrop to close */}
      {open && (
        <div className="fixed inset-0 z-40" onClick={() => setOpen(false)} />
      )}
    </div>
  );
}
