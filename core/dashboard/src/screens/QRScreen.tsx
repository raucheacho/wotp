import { useEffect, useState } from 'react';
import { useStore } from '../store';
import { useWebSocket } from '../hooks/useWebSocket';
import type { WsEvent } from '../types';
import { AlertTriangle, ScanLine } from 'lucide-react';
import { Card } from '../components/ui/card';

export default function QRScreen() {
  const [qrCode, setQrCode] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const setConnectionStatus = useStore(state => state.setConnectionStatus);

  useWebSocket({
    url: '/v1/ws/events',
    onMessage: (data: unknown) => {
      const event = data as WsEvent;
      if (event.type === 'session.reconnected') {
        setConnectionStatus('connected');
      }
    },
  });

  useEffect(() => {
    let mounted = true;
    let pollInterval: ReturnType<typeof setInterval>;

    const checkQr = async () => {
      try {
        const res = await fetch('/v1/qr');
        if (!mounted) return;
        
        if (res.status === 200) {
          const data = await res.json();
          if (data.status === 'connected') {
            setConnectionStatus('connected');
            return;
          }
          if (data.qr) {
            setQrCode(data.qr);
            setError(null);
          }
        } else {
          setError('Failed to fetch QR code');
        }
      } catch (err) {
        if (mounted) setError('Connection error');
      }
    };

    checkQr();
    pollInterval = setInterval(checkQr, 5000);

    return () => {
      mounted = false;
      clearInterval(pollInterval);
    };
  }, [setConnectionStatus]);

  return (
    <div className="min-h-screen bg-background flex flex-col items-center justify-center p-6">
      <Card className="w-full max-w-3xl flex flex-col md:flex-row overflow-hidden border-border/50 shadow-2xl">
        {/* Left side: Instructions */}
        <div className="flex-1 p-8 md:p-12 flex flex-col justify-center bg-muted/10 border-b md:border-b-0 md:border-r border-border/50">
          <div className="mb-8">
            <h2 className="text-3xl font-bold tracking-tight mb-2">Connect WhatsApp</h2>
            <p className="text-muted-foreground">
              Link your device to start sending OTPs and messages instantly.
            </p>
          </div>
          
          <div className="space-y-6">
            <div className="flex gap-4 items-start">
              <div className="w-8 h-8 rounded-full bg-primary/10 text-primary flex items-center justify-center shrink-0 font-bold">1</div>
              <div>
                <p className="font-medium">Open WhatsApp on your phone</p>
              </div>
            </div>
            <div className="flex gap-4 items-start">
              <div className="w-8 h-8 rounded-full bg-primary/10 text-primary flex items-center justify-center shrink-0 font-bold">2</div>
              <div>
                <p className="font-medium">Tap Menu or Settings and select Linked Devices</p>
              </div>
            </div>
            <div className="flex gap-4 items-start">
              <div className="w-8 h-8 rounded-full bg-primary/10 text-primary flex items-center justify-center shrink-0 font-bold">3</div>
              <div>
                <p className="font-medium">Tap on Link a Device</p>
              </div>
            </div>
            <div className="flex gap-4 items-start">
              <div className="w-8 h-8 rounded-full bg-primary/10 text-primary flex items-center justify-center shrink-0 font-bold">4</div>
              <div>
                <p className="font-medium">Point your phone to this screen to capture the code</p>
              </div>
            </div>
          </div>
        </div>

        {/* Right side: QR Code */}
        <div className="flex-1 p-8 md:p-12 flex flex-col items-center justify-center bg-background">
          <div className="relative mb-8">
            {qrCode ? (
              <div className="bg-white p-4 rounded-xl shadow-sm border">
                <img src={qrCode} alt="WhatsApp QR Code" className="w-64 h-64 object-contain" />
              </div>
            ) : error ? (
              <div className="w-64 h-64 flex flex-col items-center justify-center bg-muted/50 rounded-xl border border-dashed border-destructive/50">
                <AlertTriangle className="w-10 h-10 text-destructive mb-3" />
                <span className="text-muted-foreground font-medium text-center px-4">{error}</span>
              </div>
            ) : (
              <div className="w-64 h-64 bg-muted/30 rounded-xl border flex items-center justify-center relative overflow-hidden">
                <div className="absolute inset-0 bg-gradient-to-r from-transparent via-muted/50 to-transparent animate-[shimmer_2s_infinite]"></div>
                <ScanLine className="w-12 h-12 text-muted-foreground opacity-50" />
              </div>
            )}
            
            {/* Corner accents */}
            <div className="absolute -top-4 -left-4 w-8 h-8 border-t-2 border-l-2 border-primary rounded-tl-lg"></div>
            <div className="absolute -top-4 -right-4 w-8 h-8 border-t-2 border-r-2 border-primary rounded-tr-lg"></div>
            <div className="absolute -bottom-4 -left-4 w-8 h-8 border-b-2 border-l-2 border-primary rounded-bl-lg"></div>
            <div className="absolute -bottom-4 -right-4 w-8 h-8 border-b-2 border-r-2 border-primary rounded-br-lg"></div>
          </div>

          <div className="flex items-center gap-3 bg-muted/50 px-4 py-2 rounded-full">
            <div className="w-2.5 h-2.5 rounded-full bg-yellow-500 animate-pulse"></div>
            <span className="text-sm font-medium text-muted-foreground">Waiting for connection...</span>
          </div>
        </div>
      </Card>
      
      {/* Required for the shimmer animation if we aren't extending Tailwind config */}
      <style>{`
        @keyframes shimmer {
          0% { transform: translateX(-100%); }
          100% { transform: translateX(100%); }
        }
      `}</style>
    </div>
  );
}
