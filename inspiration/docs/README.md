# Dashboard Documentation

Dashboard Next.js 15 pour la gestion des comptes WhatsApp.

## Documentation

- **[Architecture](./architecture.md)** - Structure du projet et flux de données
- **[Authentication](./authentication.md)** - Better-Auth, sessions et sécurité
- **[WhatsApp API](./whatsapp-api.md)** - Intégration avec baileys-server
- **[Networking](./networking.md)** - Guide Docker & localhost
- **[n8n Integration](./n8n-integration.md)** - Automatisation avec n8n

## Quick Start

```bash
cd dashboard
pnpm install
pnpm dev
```

Dashboard disponible sur http://localhost:3000

## Environment Variables

```bash
# .env.local
PORT=3000

# Backend API
NEXT_PUBLIC_API_URL=http://localhost:8000
NEXT_PUBLIC_WS_URL=ws://localhost:8000
NEXT_PUBLIC_API_KEY=your-admin-api-key

# Database
DATABASE_URL=postgresql://...

# Auth
NEXT_PUBLIC_APP_URL=http://localhost:3000
```

