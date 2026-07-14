import type { Account, Stats } from "@/types";
import { create } from "zustand";

type AppState = {
  // Data
  stats?: Stats;
  currentAccount?: Account;

  // UI State
  refreshing: boolean;

  // Actions
  setStats: (stats: Stats) => void;
  setCurrentAccount: (account: Account | undefined) => void;
  setRefreshing: (refreshing: boolean) => void;

  // Callbacks (pour les actions navbar)
  onRefresh?: () => void;
  setOnRefresh: (fn: () => void) => void;
};

export const useAppStore = create<AppState>((set) => ({
  // Initial state
  stats: undefined,
  currentAccount: undefined,
  refreshing: false,
  onRefresh: undefined,

  // Actions
  setStats: (stats) => set({ stats }),
  setCurrentAccount: (currentAccount) => set({ currentAccount }),
  setRefreshing: (refreshing) => set({ refreshing }),
  setOnRefresh: (onRefresh) => set({ onRefresh }),
}));
