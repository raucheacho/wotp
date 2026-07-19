import { useStore } from '../store';
import { timeAgo } from '../utils';
import { Webhook, CheckCircle, XCircle } from 'lucide-react';
import { Card } from '../components/ui/card';

export default function WebhooksScreen() {
  const webhookEvents = useStore(state => state.webhookEvents);

  return (
    <div className="p-6 max-w-6xl mx-auto space-y-6">
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 mb-8">
        <div className="flex flex-col gap-2">
          <h2 className="text-3xl font-bold tracking-tight">Webhooks</h2>
          <p className="text-muted-foreground text-lg">Real-time HTTP callbacks for message events</p>
        </div>
      </div>

      <Card className="p-6">
        <h3 className="text-lg font-semibold mb-4">Configuration</h3>
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between p-4 bg-muted/50 rounded-lg border">
          <div className="mb-4 sm:mb-0">
            <div className="text-sm text-muted-foreground">
              Les webhooks sont configurés par projet, pas dans <code className="text-primary font-mono bg-primary/10 px-1 rounded">config.toml</code> (settings instance-wide uniquement).
              Pas encore d'écran dédié ici — utilisez l'API en attendant :
            </div>
            <pre className="mt-3 p-3 bg-background rounded border font-mono text-xs text-muted-foreground overflow-x-auto">
{`curl -X PATCH http://localhost:54321/v1/projects/<id>/settings \\
  -H "apikey: <root key>" \\
  -d '{"webhooks": {"endpoint": "https://votre-domaine.com/webhook",
                    "secret": "votre_cle_secrete",
                    "events": ["message.received", "message.delivered"]}}'`}
            </pre>
          </div>
        </div>
      </Card>

      <div className="mt-8 mb-4">
        <h3 className="text-xl font-semibold">Recent Deliveries</h3>
      </div>

      <Card className="p-4">
        {webhookEvents.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-12 text-muted-foreground text-center">
            <Webhook className="w-12 h-12 mb-4 opacity-50" />
            <h3 className="text-lg font-semibold text-foreground mb-2">Waiting for events...</h3>
            <p className="max-w-sm">
              Events will appear here once your endpoint is active.
            </p>
          </div>
        ) : (
          <div className="space-y-2">
            {webhookEvents.map((evt) => (
              <div
                key={evt.id}
                className="flex flex-col sm:flex-row sm:items-center justify-between p-4 bg-muted/30 rounded-lg hover:bg-muted/50 transition-colors gap-4"
              >
                <div className="flex items-center gap-4 flex-1">
                  <div className={`flex items-center justify-center px-2 py-1 rounded text-xs font-bold ${
                    evt.status === 'success' || evt.statusCode >= 200 && evt.statusCode < 300
                      ? 'bg-[#25D366]/10 text-[#25D366]' 
                      : 'bg-destructive/10 text-destructive'
                  }`}>
                    {evt.statusCode}
                  </div>
                  <div className="flex flex-col min-w-0 flex-1">
                    <span className="font-semibold text-sm capitalize">{evt.event.replace(/_/g, ' ')}</span>
                    {evt.url && (
                      <span className="text-xs text-muted-foreground font-mono truncate max-w-md">{evt.url}</span>
                    )}
                  </div>
                </div>

                <div className="flex items-center gap-4 sm:justify-end">
                  <div className={`flex items-center gap-1.5 text-sm font-medium capitalize ${
                    evt.status === 'success' ? 'text-[#25D366]' : 'text-destructive'
                  }`}>
                    {evt.status === 'success' ? <CheckCircle className="w-4 h-4" /> : <XCircle className="w-4 h-4" />}
                    {evt.status}
                  </div>
                  <div className="text-xs text-muted-foreground whitespace-nowrap min-w-[80px] text-right">
                    {timeAgo(evt.timestamp)}
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}
