# Intégration n8n

Automatiser les réponses WhatsApp avec n8n.

> ⚠️ Voir [Networking Guide](./networking.md) pour les URLs selon votre environnement.

## Architecture

```
WhatsApp → baileys-server → Webhook → n8n → AI Agent → HTTP Request → baileys-server → WhatsApp
```

## 1. Recevoir des messages

### Créer un Webhook dans n8n

1. Ajouter un node **Webhook**
2. Method: `POST`
3. Path: `whatsapp`

### URLs du webhook

| Mode                         | URL                                           |
| ---------------------------- | --------------------------------------------- |
| Test (Listen for Event)      | `http://localhost:5678/webhook-test/whatsapp` |
| Production (workflow activé) | `http://localhost:5678/webhook/whatsapp`      |

### Configurer baileys-server

```bash
# .envrc
WA_WEBHOOK_URL="http://localhost:5678/webhook/whatsapp"
```

### Format des événements reçus

**Message reçu :**

```json
{
  "type": "message.received",
  "timestamp": "2024-01-15T10:30:00.000Z",
  "accountId": "uuid-du-compte",
  "payload": {
    "from": "5511999999999",
    "text": "Bonjour !",
    "messageType": "text",
    "messageId": "ABC123"
  }
}
```

**Compte connecté :**

```json
{
  "type": "account.connected",
  "accountId": "uuid",
  "payload": {
    "phoneNumber": "5511999999999"
  }
}
```

---

## 2. Envoyer des messages

### URL selon l'environnement

| Environnement             | URL                                                          |
| ------------------------- | ------------------------------------------------------------ |
| n8n local                 | `http://localhost:8000/v1/accounts/{id}/messages`            |
| n8n Docker, baileys local | `http://host.docker.internal:8000/v1/accounts/{id}/messages` |
| Tout dans Docker          | `http://baileys-server:8000/v1/accounts/{id}/messages`       |

### HTTP Request Node

**Configuration :**

- Method: `POST`
- URL: `http://host.docker.internal:8000/v1/accounts/{{ $json.accountId }}/messages`
- Authentication: Header Auth
  - Name: `X-API-Key`
  - Value: `your-secret-api-key`
- Body (JSON):

```json
{
  "to": "{{ $json.payload.from }}",
  "type": "text",
  "content": "Votre réponse ici"
}
```

---

## 3. Workflow Auto-Réponse

```
[Webhook] → [IF: message.received] → [HTTP Request]
```

### IF Node

- Condition: `{{ $json.type }}` equals `message.received`

### HTTP Request

```json
{
  "to": "{{ $('Webhook').item.json.payload.from }}",
  "type": "text",
  "content": "Merci pour votre message !"
}
```

---

## 4. Workflow avec AI Agent

```
[Webhook] → [IF] → [AI Agent] → [HTTP Request]
```

### AI Agent Node

- Model: GPT-4 ou autre
- System prompt: "Tu es un assistant WhatsApp professionnel."
- User message: `{{ $json.payload.text }}`

### HTTP Request (après AI)

```json
{
  "to": "{{ $('Webhook').item.json.payload.from }}",
  "type": "text",
  "content": "{{ $json.output }}"
}
```

---

## 5. Endpoints utiles

### Lister les comptes

```
GET http://host.docker.internal:8000/v1/accounts
X-API-Key: your-secret-api-key
```

### Vérifier le statut

```
GET http://host.docker.internal:8000/v1/accounts/{id}/status
```

### Déconnecter

```
POST http://host.docker.internal:8000/v1/accounts/{id}/disconnect
```

---

## 6. Docker Compose

```yaml
services:
  n8n:
    image: n8nio/n8n
    ports:
      - "5678:5678"
    environment:
      - WEBHOOK_URL=http://n8n:5678/
    networks:
      - app-network

  baileys-server:
    build: ./baileys-server
    ports:
      - "8000:8000"
    environment:
      - WA_WEBHOOK_URL=http://n8n:5678/webhook/whatsapp
      - WA_API_KEYS=your-secret-api-key
      - REDIS_URL=redis://redis:6379
    networks:
      - app-network

  redis:
    image: redis:7-alpine
    networks:
      - app-network

networks:
  app-network:
```

---

## 7. Debugging

### Tester le webhook manuellement

```bash
curl -X POST http://localhost:5678/webhook/whatsapp \
  -H "Content-Type: application/json" \
  -d '{
    "type": "message.received",
    "accountId": "test-id",
    "payload": {
      "from": "5511999999999",
      "text": "Test"
    }
  }'
```

### Tester l'envoi

```bash
curl -X POST http://localhost:8000/v1/accounts/{id}/messages \
  -H "X-API-Key: your-secret-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "5511999999999",
    "type": "text",
    "content": "Hello!"
  }'
```
