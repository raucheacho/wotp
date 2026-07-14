# Chat Gate Dashboard

Admin dashboard for managing the WhatsApp Transport Gateway. Built with Next.js 15, React 19, and Tailwind CSS.

## Features

- 🔐 **Authentication** - Better-Auth with email/password
- 🏢 **Organizations management** - Create, edit, delete SaaS clients
- 📱 **Accounts management** - WhatsApp numbers per organization
- 🔑 **API Key display** - Copy organization API keys
- ⚙️ **Webhook configuration** - URL, secret, event filters
- 📈 **Statistics** - Connected accounts, messages, errors
- 📝 **Audit Logs** - Track all actions
- 📊 **Webhook Stats** - Delivery success rates
- 🎨 **Dark theme** - Modern WhatsApp-inspired design

## Screenshots

### Home - Dashboard Overview

- Organization details card
- Connected accounts stats
- Quick actions

### Organization Detail

- List of WhatsApp accounts
- Create new accounts
- Edit organization settings
- Configure webhook event filters

### Account Detail

- Connection status
- QR code for authentication
- Message logs and errors

## Quick Start

```bash
cd dashboard
pnpm install
cp .env.example .env.local
pnpm dev
```

Open [http://localhost:3000](http://localhost:3000)

## Environment Variables

```env
# Backend API URL
NEXT_PUBLIC_API_URL=http://localhost:8000

# Admin API key (for direct backend calls)
NEXT_PUBLIC_API_KEY=your-admin-api-key

# Database (same as backend)
DATABASE_URL=postgresql://...

# Auth
NEXT_PUBLIC_APP_URL=http://localhost:3000
```

## Project Structure

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
│   ├── admin/                      # Admin panel
│   └── api/
│       ├── auth/[...all]/          # Better-Auth handler
│       ├── organizations/          # Organization CRUD
│       ├── accounts/               # Account operations
│       ├── baileys-proxy/          # Proxy to baileys-server
│       ├── webhooks/               # Webhook management
│       └── audit/                  # Audit logs
├── components/
│   ├── ui/                         # shadcn/ui components
│   ├── AccountList.tsx             # Accounts grid
│   ├── AccountModal.tsx            # Create/edit account
│   ├── AccountApiKeys.tsx          # API key display
│   ├── AccountInfoCards.tsx        # Account stats
│   ├── AuditLogs.tsx               # Audit trail viewer
│   ├── CodeSnippetsModal.tsx       # API code examples
│   ├── ConnectionLogs.tsx          # Connection history
│   ├── DashboardStats.tsx          # Overview stats
│   ├── MessageLogs.tsx             # Message history
│   ├── OrganizationList.tsx        # Organizations grid
│   ├── OrganizationModal.tsx       # Create organization
│   ├── OrganizationEditModal.tsx   # Edit organization
│   ├── OrganizationDetailsCard.tsx # Org info display
│   ├── OrganizationSwitcher.tsx    # Switch between orgs
│   ├── QuotaIndicator.tsx          # Usage quotas
│   ├── WebhookSettingsCard.tsx     # Webhook configuration
│   ├── WebhookStats.tsx            # Webhook delivery stats
│   ├── WebhookTest.tsx             # Test webhook
│   ├── SideBar.tsx                 # Navigation
│   └── Navbar.tsx                  # Top bar
├── lib/
│   ├── api-client.ts               # TypeScript API client
│   ├── auth.ts                     # Better-Auth config
│   ├── auth-client.ts              # Client-side auth
│   ├── auth-schema.ts              # Auth DB schema
│   ├── db.ts                       # Drizzle connection
│   ├── store.ts                    # Organization store
│   └── config.ts                   # Environment config
├── stores/
│   └── useAppStore.ts              # Zustand store
├── types/
│   └── index.ts                    # TypeScript types
└── docs/                           # Documentation
```

## Key Features

### Organization Management

```typescript
// Create organization
POST /api/organizations
{
  "name": "My SaaS",
  "webhookUrl": "https://...",
  "plan": "pro",
  "maxAccounts": 100
}

// Update with event filters
PATCH /api/organizations/:id
{
  "metadata": {
    "webhookFilter": {
      "events": ["message.received", "account.connected"],
      "includeOwnMessages": false
    }
  }
}
```

### Webhook Event Filters

Configure which events each organization receives:

- **message.received** - Incoming messages (default: enabled)
- **message.sent** - Sent confirmations (default: disabled)
- **message.failed** - Failed messages (default: disabled)
- **account.connected** - Connection events (default: enabled)
- **account.disconnected** - Disconnection events (default: enabled)

Additional filters:

- Include group messages
- Include broadcast messages
- Include own messages (fromMe)

## Development

```bash
# Run development server
pnpm dev

# Type check
pnpm tsc --noEmit

# Build for production
pnpm build
```

## Tech Stack

- **Framework**: Next.js 15 (App Router)
- **UI**: React 19, Tailwind CSS
- **State**: Zustand
- **Database**: Drizzle ORM + PostgreSQL
- **Icons**: Lucide React

## License

MIT
