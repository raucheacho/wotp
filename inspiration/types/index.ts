/**
 * Type definitions for WhatsApp Automation Platform
 * These types are re-exported from the API client to ensure consistency with the server
 */

export type {
    Account, AuditLog, MessageError, MessageLog, Organization, WebhookDelivery
} from "@/lib/api-client";

// Legacy type aliases for backward compatibility
export type OrganizationPlan = "starter" | "pro" | "enterprise";
export type AccountStatus = "disconnected" | "connecting" | "connected" | "error";
export type MessageDirection = "sent" | "received";
export type MessageLogStatus = "queued" | "sent" | "delivered" | "failed";

// Request types for forms
export interface CreateOrganizationRequest {
  name: string;
  webhookUrl?: string;
  webhookSecret?: string;
  plan?: OrganizationPlan;
  maxAccounts?: number;
  defaultRateLimit?: number;
  metadata?: Record<string, unknown>;
}

export interface CreateAccountRequest {
  organizationId: string;
  name: string;
  webhookUrl?: string;
  webhookSecret?: string;
  rateLimit?: number;
  metadata?: Record<string, unknown>;
}

// Stats types (extended for dashboard)
export interface Stats {
  total: number;
  connected: number;
  disconnected: number;
  error: number;
  webhookDeliveries?: {
    total: number;
    successful: number;
    failed: number;
    averageAttempts: number;
  };
  auditLogs?: {
    total: number;
    recentErrors: number;
  };
}

// Error types


// Status configuration for UI
export const statusConfig: Record<
  AccountStatus,
  { label: string; color: string; bg: string }
> = {
  disconnected: {
    label: "Déconnecté",
    color: "text-zinc-500",
    bg: "bg-zinc-800",
  },
  connecting: {
    label: "Connexion...",
    color: "text-yellow-500",
    bg: "bg-yellow-500/10",
  },
  connected: {
    label: "Connecté",
    color: "text-[#25D366]",
    bg: "bg-[#25D366]/10",
  },
  error: { label: "Erreur", color: "text-red-500", bg: "bg-red-500/10" },
};

// Message log status configuration for UI
export const logStatusConfig: Record<
  MessageLogStatus,
  { label: string; color: string }
> = {
  queued: { label: "En attente", color: "text-yellow-500" },
  sent: { label: "Envoyé", color: "text-blue-500" },
  delivered: { label: "Délivré", color: "text-[#25D366]" },
  failed: { label: "Échoué", color: "text-red-500" },
};
