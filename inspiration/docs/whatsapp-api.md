# WhatsApp API Integration

Comment le dashboard communique avec baileys-server.

## Configuration

```bash
# dashboard/.env.local
NEXT_PUBLIC_API_URL=http://localhost:8000
NEXT_PUBLIC_WS_URL=ws://localhost:8000
NEXT_PUBLIC_API_KEY=your-admin-api-key
```

## API Client

Le dashboard utilise un client TypeScript centralisé (`lib/api-client.ts`) pour toutes les communications avec baileys-server.

```typescript
import { apiClient } from "@/lib/api-client";
```

---

## Organizations API

### Lister les organisations

```typescript
const organizations = await apiClient.listOrganizations();
// Returns: Organization[]
```

### Créer une organisation

```typescript
const org = await apiClient.createOrganization({
  name: "My SaaS Platform",
  webhookUrl: "https://my-saas.com/webhooks",
  webhookSecret: "secret-for-hmac",
  plan: "pro", // starter | pro | enterprise
  maxAccounts: 100,
  defaultRateLimit: 30,
});
// Returns: Organization (includes apiKey)
```

### Mettre à jour une organisation

```typescript
const updated = await apiClient.updateOrganization(orgId, {
  name: "New Name",
  webhookUrl: "https://new-url.com/webhooks",
  plan: "enterprise",
  maxAccounts: 500,
});
```

### Supprimer une organisation

```typescript
await apiClient.deleteOrganization(orgId);
// Cascade delete: supprime aussi tous les accounts
```

---

## Accounts API

### Lister les comptes

```typescript
// Tous les comptes (admin)
const accounts = await apiClient.listAccounts();

// Comptes d'une organisation
const accounts = await apiClient.listAccounts(organizationId);
```

### Créer un compte

```typescript
const account = await apiClient.createAccount({
  organizationId: "uuid-org",
  name: "Restaurant Bot",
  webhookUrl: "https://...", // Override org webhook
  rateLimit: 50,
  metadata: { userId: "user-123" },
});
// account.publicToken pour le WebSocket QR
```

### Obtenir un compte

```typescript
const account = await apiClient.getAccount(accountId);
```

### Mettre à jour un compte

```typescript
const updated = await apiClient.updateAccount(accountId, {
  name: "New Name",
  webhookUrl: null, // Use org webhook
  rateLimit: 100,
});
```

### Supprimer un compte

```typescript
await apiClient.deleteAccount(accountId);
```

### Régénérer le token public

```typescript
const account = await apiClient.regenerateAccountToken(accountId);
// account.publicToken is now a new value
```

---

## Connection API

### Démarrer la connexion

```typescript
const result = await apiClient.connectAccount(accountId);
// { message: "Connection initiated", accountId, status: "connecting" }
```

### Obtenir le QR code

```typescript
const { qr, accountId } = await apiClient.getQRCode(accountId);
// qr: base64 encoded QR code image
```

### Obtenir le statut

```typescript
const status = await apiClient.getAccountStatus(accountId);
// { status: "connected", phoneNumber: "+33...", webhookUrl, rateLimit }
```

### Déconnecter

```typescript
await apiClient.disconnectAccount(accountId);
```

---

## Messages API

### Envoyer un message texte

```typescript
const { messageId } = await apiClient.sendTextMessage(accountId, {
  to: "5511999999999",
  text: "Hello!",
  replyToMessageId: "optional-msg-id", // Pour répondre
});
```

### Obtenir les logs de messages

```typescript
const { logs } = await apiClient.getMessageLogs(accountId, {
  limit: 50,
  offset: 0,
  direction: "sent", // sent | received
  status: "delivered", // queued | sent | delivered | failed
});
```

### Obtenir les erreurs

```typescript
const { errors } = await apiClient.getMessageErrors(accountId, 50);
```

### Obtenir les statistiques

```typescript
const stats = await apiClient.getMessageStats(accountId);
// { accountId, messageCount: 150, errorCount: 3 }
```

---

## Monitoring API

### Logs d'audit

```typescript
const auditLogs = await apiClient.getAuditLogs({
  organizationId: "uuid",
  action: "create_account,delete_account",
  resourceType: "account",
  fromDate: new Date("2024-01-01"),
  limit: 100,
});
```

### Logs de connexion

```typescript
const connectionLogs = await apiClient.getConnectionLogs(accountId, 20);
// Filtré sur les actions connect/disconnect
```

### Webhook deliveries

```typescript
const deliveries = await apiClient.getWebhookDeliveries({
  organizationId: "uuid",
  event: "message.received",
  success: true,
  fromDate: new Date("2024-01-01"),
  limit: 50,
});
```

### Statistiques webhooks

```typescript
const stats = await apiClient.getWebhookDeliveryStats(organizationId, 30);
// { total: 1000, successful: 980, failed: 20, averageAttempts: 1.02 }
```

---

## WebSocket pour QR Code

Le dashboard utilise le `publicToken` pour se connecter au WebSocket sans exposer l'API key côté client.

```typescript
const ws = new WebSocket(`${WS_URL}/ws/${account.publicToken}`);

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);

  switch (data.type) {
    case "qr":
      setQrCode(data.qr);
      break;
    case "status":
      setStatus(data.status);
      break;
    case "connected":
      setPhoneNumber(data.phoneNumber);
      ws.close();
      break;
  }
};
```

### Sequence Diagram

```
Dashboard                    baileys-server
   |                              |
   |-- GET /v1/accounts --------->|
   |<-- { accounts, publicToken } |
   |                              |
   |-- WS /ws/{publicToken} ----->|
   |<-- { type: "qr", qr: "..." } |
   |                              |
   | [User scans QR]              |
   |                              |
   |<-- { type: "connected" } ----|
```

---

## Types TypeScript

```typescript
interface Organization {
  id: string;
  name: string;
  apiKey: string;
  plan: "starter" | "pro" | "enterprise";
  maxAccounts: number;
  defaultRateLimit: number;
  webhookUrl?: string;
  webhookSecret?: string;
  metadata?: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
}

interface Account {
  id: string;
  organizationId: string;
  name: string;
  status: "disconnected" | "connecting" | "connected" | "error";
  phoneNumber?: string;
  publicToken: string;
  webhookUrl?: string;
  webhookSecret?: string;
  rateLimit: number;
  metadata?: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
}

interface MessageLog {
  id: string;
  accountId: string;
  direction: "sent" | "received";
  phoneNumber: string;
  messageType: string;
  contentPreview?: string;
  status: "queued" | "sent" | "delivered" | "failed";
  error?: string;
  timestamp: string;
}

interface AuditLog {
  id: string;
  organizationId?: string;
  action: string;
  resourceType: string;
  resourceId?: string;
  resourceName?: string;
  success: boolean;
  errorMessage?: string;
  timestamp: string;
}

interface WebhookDelivery {
  id: string;
  organizationId: string;
  event: string;
  url: string;
  success: boolean;
  attempts: number;
  statusCode?: number;
  errorMessage?: string;
  createdAt: string;
  deliveredAt?: string;
}
```

---

## Gestion des erreurs

```typescript
try {
  const account = await apiClient.getAccount(id);
} catch (error) {
  // error.message contains the API error message
  // Common error codes:
  // - AUTH_REQUIRED (401)
  // - AUTH_INVALID (401)
  // - FORBIDDEN (403)
  // - NOT_FOUND (404)
  // - VALIDATION_ERROR (400)
  // - ACCOUNT_DISCONNECTED (422)
  // - RATE_LIMITED (429)
}
```

