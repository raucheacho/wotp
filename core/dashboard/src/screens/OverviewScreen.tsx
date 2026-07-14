import { useStore } from '../store';
import { Card } from '../components/ui/card';
import { Activity, CheckCircle, Zap, Link } from 'lucide-react';

export default function OverviewScreen() {
  const stats = useStore(state => state.stats);
  const connectionStatus = useStore(state => state.connectionStatus);

  return (
    <div className="p-6 max-w-6xl mx-auto space-y-6">
      <div className="flex flex-col gap-2 mb-8">
        <h2 className="text-3xl font-bold tracking-tight">Welcome to Wotp!</h2>
        <p className="text-muted-foreground text-lg">Your Developer Dashboard for OTP and Messaging via WhatsApp.</p>
      </div>

      <Card className="p-6">
        <div className="flex items-center gap-4">
          <div className={`p-3 rounded-full flex items-center justify-center ${connectionStatus === 'connected' ? 'bg-[#25D366]/10 text-[#25D366]' : 'bg-destructive/10 text-destructive'}`}>
            <Activity className="w-6 h-6" />
          </div>
          <div>
            <h3 className="text-lg font-semibold">Engine Status: {connectionStatus === 'connected' ? 'Online' : 'Offline'}</h3>
            <p className="text-muted-foreground text-sm">
              The WhatsApp Engine is currently {connectionStatus === 'connected' ? 'connected and ready to send messages' : 'disconnected or syncing'}.
            </p>
          </div>
        </div>
      </Card>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <Card className="p-4">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-blue-500/10 text-blue-500">
              <Zap className="w-4 h-4" />
            </div>
            <div>
              <p className="text-xs text-muted-foreground">Total OTPs Today</p>
              <p className="text-lg font-semibold text-foreground">{stats.messagesToday}</p>
            </div>
          </div>
        </Card>
        
        <Card className="p-4">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-[#25D366]/10 text-[#25D366]">
              <CheckCircle className="w-4 h-4" />
            </div>
            <div>
              <p className="text-xs text-muted-foreground">Delivery Success</p>
              <p className="text-lg font-semibold text-foreground">{stats.successRate}%</p>
            </div>
          </div>
        </Card>
        
        <Card className="p-4">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-yellow-500/10 text-yellow-500">
              <Activity className="w-4 h-4" />
            </div>
            <div>
              <p className="text-xs text-muted-foreground">Avg Latency</p>
              <p className="text-lg font-semibold text-foreground">{stats.avgResponseMs > 0 ? `${stats.avgResponseMs}ms` : '< 1s'}</p>
            </div>
          </div>
        </Card>
      </div>
      
      <Card className="p-6">
        <h3 className="text-lg font-semibold mb-2">Quick Start</h3>
        <p className="text-muted-foreground text-sm mb-4">Send your first OTP via cURL:</p>
        <div className="bg-muted p-4 rounded-md font-mono text-sm text-foreground overflow-x-auto whitespace-pre">
          {`curl -X POST http://localhost:8080/api/otp/send \\
  -H "Content-Type: application/json" \\
  -d '{"phone":"+1234567890"}'`}
        </div>
      </Card>
    </div>
  );
}
