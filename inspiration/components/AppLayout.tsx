"use client";
import { Navbar } from "@/components/Navbar";
import { SideBar } from "@/components/SideBar";
import { useAppStore } from "@/stores/useAppStore";

type AppLayoutProps = {
  children: React.ReactNode;
  showNewButton?: boolean;
};

export function AppLayout({ children }: AppLayoutProps) {
  const { stats, currentAccount, refreshing, onRefresh } =
    useAppStore();

  return (
    <div className="min-h-screen bg-background text-foreground">
      <Navbar />

      <div className="pt-14 flex max-w-6xl mx-auto">
        <SideBar />
        <main className="flex-1 p-6">{children}</main>
      </div>
    </div>
  );
}
