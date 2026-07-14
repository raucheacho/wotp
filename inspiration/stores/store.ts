import { create } from "zustand";
import { persist } from "zustand/middleware";

export interface Organization {
  [x: string]: unknown;
  id: string;
  name: string;
  slug?: string;
  logo?: string | null;
  metadata?: string;
  createdAt?: Date;
}

interface OrganizationState {
  activeOrganization: Organization | null;
  setActiveOrganization: (org: Organization | null) => void;
}

export const useOrganizationStore = create<OrganizationState>()(
  persist(
    (set) => ({
      activeOrganization: null,
      setActiveOrganization: (org) => set({ activeOrganization: org }),
    }),
    {
      name: "organization-storage",
    },
  ),
);
