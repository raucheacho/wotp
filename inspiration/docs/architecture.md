# Architecture

## Stack

- **Framework:** Next.js 15 (App Router)
- **Authentication:** Better-Auth (organization + admin plugins)
- **Database:** Neon PostgreSQL + Drizzle ORM
- **State Management:** Zustand
- **Styling:** Tailwind CSS + shadcn/ui
- **WhatsApp:** baileys-server (service externe)

## Structure

```
dashboard/
├── app/
│   ├── page.tsx                    # Dashboard overview
│   ├── accounts/[id]/              # Account detail
│   ├── organizations/[id]/         # Organization detail
│   ├── connect/[token]/            # Public QR code page
│   ├── billing/                    # Billing & plans
│   ├── settings/                   # Settings page
│   ├── api-docs/                   # API & Webhooks config
│   ├── auth/                       # Sign in/up pages
│   ├── onboarding/                 # New user onboarding
│   └── api/
│       ├── auth/[...all]/          # Better-Auth routes
│       ├── organizations/          # Organization API
│       ├── accounts/               # Accounts API
│       ├── baileys-proxy/          # Proxy to baileys-server
│       └── webhooks/               # Webhook management
├── components/
│   ├── ui/                         # shadcn/ui components
│   ├── AccountList.tsx             # Accounts grid
│   ├── AccountModal.tsx            # Create/edit account
│   ├── OrganizationList.tsx        # Organizations grid
│   ├── OrganizationModal.tsx       # Create organization
│   ├── OrganizationEditModal.tsx   # Edit organization
│   ├── WebhookSettingsCard.tsx     # Webhook configuration
│   ├── WebhookStats.tsx            # Webhook delivery stats
│   ├── AuditLogs.tsx               # Audit trail viewer
│   ├── MessageLogs.tsx             # Message history
│   ├── SideBar.tsx                 # Navigation
│   └── Navbar.tsx                  # Top bar
├── lib/
│   ├── api-client.ts               # TypeScript API client
│   ├── auth.ts                     # Better-Auth config
│   ├── auth-schema.ts              # Auth DB schema
│   ├── db.ts                       # Drizzle connection
│   ├── store.ts                    # Zustand store
│   └── config.ts                   # Environment config
├── stores/
│   └── useAppStore.ts              # Global state
├── types/
│   └── index.ts                    # TypeScript types
└── docs/                           # Documentation
```

## Flux de données

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              FRONTEND                                    │
│  ┌─────────────┐     ┌──────────────────┐     ┌─────────────────────┐  │
│  │  Dashboard  │────▶│  API Client      │────▶│  Next.js API Routes │  │
│  │  (React)    │     │  (TypeScript)    │     │  (Proxy + Auth)     │  │
│  └─────────────┘     └──────────────────┘     └──────────┬──────────┘  │
└───────────────────────────────────────────────────────────┼─────────────┘
                                                            │
                                                            ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                              BACKEND                                     │
│  ┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐   │
│  │  baileys-server │────▶│  WhatsApp       │────▶│  Webhooks       │   │
│  │  (Fastify)      │◀────│  (Baileys)      │     │  (n8n, etc.)    │   │
│  └────────┬────────┘     └─────────────────┘     └─────────────────┘   │
│           │                                                             │
│  ┌────────▼────────┐     ┌─────────────────┐                           │
│  │  PostgreSQL     │     │  Redis          │                           │
│  │  (Neon)         │     │  (Upstash)      │                           │
│  └─────────────────┘     └─────────────────┘                           │
└─────────────────────────────────────────────────────────────────────────┘
```

## Authentication

### Better-Auth Configuration

Le dashboard utilise [Better-Auth](https://www.better-auth.com/) avec les plugins suivants:

- **organization**: Gestion multi-tenant, chaque organization = un client SaaS
- **admin**: Administration des utilisateurs et organisations

```typescript
// lib/auth.ts
export const auth = betterAuth({
  emailAndPassword: { enabled: true },
  plugins: [
    organization({
      additionalFields: {
        apiKey: { type: "string", defaultValue: () => "sk_" + crypto.randomUUID() }
      }
    }),
    admin(),
    nextCookies(),
  ],
});
```

### Session Flow

1. User signs in via `/auth/signin`
2. Better-Auth creates session with organization context
3. API routes access session via `auth.api.getSession()`
4. Organization context determines data access

## Responsabilités

### Dashboard (ce repo)

- 🔐 Authentification et gestion des sessions
- 👥 Gestion des organisations (multi-tenant)
- 📱 UI de gestion des comptes WhatsApp
- 📊 Affichage des statistiques et logs
- ⚙️ Configuration des webhooks
- 🔌 Intégrations (n8n, etc.)

### baileys-server (repo externe)

- 📡 Transport WhatsApp pur via Baileys
- 💾 Gestion des sessions WhatsApp (Redis)
- 🔄 API REST + WebSocket pour QR codes
- 📤 Webhooks sortants vers les clients
- ⏳ Rate limiting et queues de messages

## Base de données

### PostgreSQL (Neon) - Dashboard

Tables Better-Auth:
- `user` - Utilisateurs
- `session` - Sessions actives
- `account` - Comptes OAuth (user_accounts)
- `verification` - Tokens de vérification
- `organization` - Organisations
- `member` - Membres d'organisation
- `invitation` - Invitations

### PostgreSQL (Neon) - baileys-server

Tables métier:
- `organizations` - Config des clients SaaS
- `accounts` - Comptes WhatsApp
- `message_logs` - Historique des messages
- `message_errors` - Erreurs et incidents
- `audit_logs` - Journal d'audit
- `webhook_deliveries` - Suivi des webhooks

### Redis (Upstash) - baileys-server

- Sessions WhatsApp (Baileys auth state)
- Rate limiting par compte/destination
- Queue de messages avec retry

## Communication

| Source         | Destination    | Protocole      | Usage                        |
| -------------- | -------------- | -------------- | ---------------------------- |
| Dashboard      | baileys-server | HTTP (REST)    | CRUD comptes, envoi messages |
| Dashboard      | baileys-server | WebSocket      | QR code temps réel           |
| baileys-server | Clients        | HTTP (webhook) | Events (messages reçus, etc.)|
| n8n            | baileys-server | HTTP           | Réponses automatiques        |

## API Client

Le dashboard communique avec baileys-server via `lib/api-client.ts`:

```typescript
import { apiClient } from "@/lib/api-client";

// Organizations
const orgs = await apiClient.listOrganizations();
await apiClient.createOrganization({ name: "My SaaS" });

// Accounts
const accounts = await apiClient.listAccounts(orgId);
await apiClient.connectAccount(accountId);
const { qr } = await apiClient.getQRCode(accountId);

// Messages
await apiClient.sendTextMessage(accountId, { to: "5511...", text: "Hello!" });
const { logs } = await apiClient.getMessageLogs(accountId);

// Monitoring
const auditLogs = await apiClient.getAuditLogs({ organizationId });
const deliveries = await apiClient.getWebhookDeliveries({ organizationId });
```
