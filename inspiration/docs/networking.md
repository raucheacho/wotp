# Guide Networking

Comment configurer les URLs selon votre environnement.

## Le problème de `localhost`

`localhost` signifie "cette machine" - mais sa signification change selon le contexte :

| Contexte         | `localhost` pointe vers |
| ---------------- | ----------------------- |
| Votre terminal   | Votre machine           |
| Conteneur Docker | Le conteneur lui-même   |
| Navigateur       | Votre machine           |

## Scénarios

### 1. Tout en local (sans Docker)

```bash
# .envrc
WA_WEBHOOK_URL="http://localhost:5678/webhook/whatsapp"

# dashboard/.env.local
NEXT_PUBLIC_API_URL=http://localhost:8000
```

✅ `localhost` fonctionne partout.

---

### 2. baileys-server local + n8n dans Docker

**Problème :** n8n dans Docker ne peut pas accéder à `localhost:8000`

**Solutions :**

#### Option A : `host.docker.internal` (macOS/Windows)

```
# Dans n8n HTTP Request node
http://host.docker.internal:8000/v1/accounts/...
```

#### Option B : IP de la machine

```bash
# Trouvez votre IP
ifconfig | grep "inet " | grep -v 127.0.0.1

# Dans n8n
http://192.168.1.100:8000/v1/accounts/...
```

**Webhook (baileys → n8n) :** fonctionne avec `localhost:5678` car baileys est sur la machine hôte.

---

### 3. Tout dans Docker (même réseau)

```yaml
# docker-compose.yml
services:
  baileys-server:
    environment:
      - WA_WEBHOOK_URL=http://n8n:5678/webhook/whatsapp
    networks:
      - app-network

  n8n:
    networks:
      - app-network

  dashboard:
    environment:
      - NEXT_PUBLIC_API_URL=http://localhost:8000 # Pour le navigateur
    networks:
      - app-network

networks:
  app-network:
```

**URLs entre services Docker :**

- `http://baileys-server:8000`
- `http://n8n:5678`

**URLs depuis le navigateur :**

- `http://localhost:8000` (ports exposés)

---

### 4. Next.js : Server vs Client

| Contexte            | Où ça tourne        | URL                          |
| ------------------- | ------------------- | ---------------------------- |
| Client (useEffect)  | Navigateur          | `http://localhost:8000`      |
| Server (API routes) | Node.js dans Docker | `http://baileys-server:8000` |

```bash
# Variables client (NEXT_PUBLIC_)
NEXT_PUBLIC_API_URL=http://localhost:8000

# Variables server-only
API_URL_INTERNAL=http://baileys-server:8000
```

---

## Résumé

| De → Vers        | Local            | Docker → Host               | Docker → Docker       |
| ---------------- | ---------------- | --------------------------- | --------------------- |
| n8n → baileys    | `localhost:8000` | `host.docker.internal:8000` | `baileys-server:8000` |
| baileys → n8n    | `localhost:5678` | `localhost:5678`            | `n8n:5678`            |
| Navigateur → API | `localhost:8000` | `localhost:8000`            | `localhost:8000`      |

---

## Debugging

```bash
# Tester depuis un conteneur
docker exec -it n8n sh
curl http://host.docker.internal:8000/health

# Vérifier les réseaux
docker network ls
docker network inspect app-network
```
