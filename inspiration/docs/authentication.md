# Authentication

Guide d'authentification du dashboard Chat Gate.

## Stack

- **Better-Auth** - Framework d'authentification pour Next.js
- **Drizzle ORM** - Adaptateur base de données
- **PostgreSQL (Neon)** - Stockage des sessions et utilisateurs

## Configuration

### Environment Variables

```bash
# .env.local
DATABASE_URL=postgresql://...        # Neon PostgreSQL
NEXT_PUBLIC_APP_URL=http://localhost:3000
```

### Better-Auth Setup

```typescript
// lib/auth.ts
import { betterAuth } from "better-auth";
import { drizzleAdapter } from "better-auth/adapters/drizzle";
import { nextCookies } from "better-auth/next-js";
import { admin, organization } from "better-auth/plugins";

export const auth = betterAuth({
  database: drizzleAdapter(db, { provider: "pg" }),
  emailAndPassword: { enabled: true },
  plugins: [
    organization({
      additionalFields: {
        apiKey: {
          type: "string",
          defaultValue: () => "sk_" + crypto.randomUUID().replace(/-/g, ""),
        },
      },
    }),
    admin(),
    nextCookies(),
  ],
});
```

## Plugins

### Organization Plugin

Gestion multi-tenant : chaque organisation = un client SaaS.

**Fonctionnalités :**
- Création/gestion d'organisations
- Membres et rôles (owner, admin, member)
- Invitations par email
- API Key auto-générée par organisation

```typescript
// Côté client
import { authClient } from "@/lib/auth-client";

// Créer une organisation
await authClient.organization.create({ name: "My Company" });

// Lister les organisations de l'utilisateur
const { data: orgs } = await authClient.organization.list();

// Changer d'organisation active
await authClient.organization.setActive({ organizationId: org.id });
```

### Admin Plugin

Permet la gestion des utilisateurs et organisations par les administrateurs.

## Routes API

### Better-Auth Handler

```typescript
// app/api/auth/[...all]/route.ts
import { auth } from "@/lib/auth";
import { toNextJsHandler } from "better-auth/next-js";

export const { GET, POST } = toNextJsHandler(auth.handler);
```

### Protected API Routes

```typescript
// app/api/protected/route.ts
import { auth } from "@/lib/auth";
import { headers } from "next/headers";

export async function GET() {
  const session = await auth.api.getSession({
    headers: await headers(),
  });

  if (!session) {
    return Response.json({ error: "Unauthorized" }, { status: 401 });
  }

  // Access organization context
  const orgId = session.session.activeOrganizationId;

  return Response.json({ userId: session.user.id, orgId });
}
```

## Flow d'authentification

```
┌──────────────┐     ┌─────────────────┐     ┌──────────────┐
│   /auth/     │────▶│  Better-Auth    │────▶│  PostgreSQL  │
│   signin     │     │  API Handler    │     │  (Neon)      │
└──────────────┘     └─────────────────┘     └──────────────┘
       │                      │
       │                      ▼
       │             ┌─────────────────┐
       │             │  Session Cookie │
       │             │  (httpOnly)     │
       │             └─────────────────┘
       │                      │
       ▼                      ▼
┌──────────────────────────────────────────┐
│              Dashboard                    │
│  - Session validée via cookie            │
│  - Organization active accessible        │
│  - API calls authentifiés                │
└──────────────────────────────────────────┘
```

## Client-Side Auth

```typescript
// lib/auth-client.ts
import { createAuthClient } from "better-auth/react";
import { organizationClient, adminClient } from "better-auth/client/plugins";

export const authClient = createAuthClient({
  baseURL: process.env.NEXT_PUBLIC_APP_URL,
  plugins: [organizationClient(), adminClient()],
});

// Hooks disponibles
export const { useSession, signIn, signOut, signUp } = authClient;
```

### Utilisation dans les composants

```tsx
"use client";
import { useSession, signOut } from "@/lib/auth-client";

export function UserMenu() {
  const { data: session, isPending } = useSession();

  if (isPending) return <Spinner />;
  if (!session) return <SignInButton />;

  return (
    <div>
      <span>{session.user.email}</span>
      <button onClick={() => signOut()}>Sign Out</button>
    </div>
  );
}
```

## Database Schema

```typescript
// lib/auth-schema.ts (généré par Better-Auth)

export const user = pgTable("user", {
  id: text("id").primaryKey(),
  name: text("name").notNull(),
  email: text("email").notNull().unique(),
  emailVerified: boolean("email_verified"),
  image: text("image"),
  createdAt: timestamp("created_at"),
  updatedAt: timestamp("updated_at"),
});

export const session = pgTable("session", {
  id: text("id").primaryKey(),
  userId: text("user_id").references(() => user.id),
  activeOrganizationId: text("active_organization_id"),
  expiresAt: timestamp("expires_at"),
});

export const organizations = pgTable("organization", {
  id: text("id").primaryKey(),
  name: text("name").notNull(),
  slug: text("slug").unique(),
  logo: text("logo"),
  apiKey: text("api_key"), // Custom field
  createdAt: timestamp("created_at"),
});
```

## Sécurité

### CORS & Trusted Origins

```typescript
// lib/auth.ts
export const auth = betterAuth({
  // ...
  trustedOrigins: [
    "http://localhost:3000",
    "https://your-production-domain.com",
  ],
});
```

### Session Validation

- Sessions stockées en base de données (pas JWT)
- Cookie httpOnly, secure en production
- Expiration configurable
- Révocation possible côté serveur
