import { useCallback, useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { Card } from '../components/ui/card';
import { AlertTriangle, ArrowLeft, CheckCircle2 } from 'lucide-react';

// Dedicated page for pairing a whatsmeow number — reached via the "Scan"
// button on the Numbers screen instead of showing the QR inline there.
// Starts pairing on mount (no separate "start" click needed once you're
// here) and polls for the QR/connection state until either succeeds.
export default function ScanScreen() {
  const [qrCode, setQrCode] = useState<string | null>(null);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const pairingStarted = useRef(false);

  const fetchQr = useCallback(async () => {
    try {
      const res = await fetch('/dashboard/api/numbers/qr', {
        headers: { Accept: 'application/json' },
      });
      if (res.status === 200) {
        const data = await res.json();
        if (data.qr) {
          const hash = btoa(data.qr).replace(/=/g, '').substring(0, 20);
          setQrCode(`/dashboard/api/numbers/qr?hash=${hash}`);
        }
      } else {
        // 503 = no pairing in progress right now — not an error, just
        // means the code expired and a new one is about to be issued.
        setQrCode(null);
      }
    } catch {
      // next poll will retry
    }
  }, []);

  const checkConnected = useCallback(async () => {
    try {
      const res = await fetch('/dashboard/api/numbers');
      if (res.ok) {
        const data: { connected: boolean }[] = await res.json();
        if (data.some((n) => n.connected)) setConnected(true);
      }
    } catch {
      // next poll will retry
    }
  }, []);

  // Start pairing exactly once per visit to this page.
  useEffect(() => {
    if (pairingStarted.current) return;
    pairingStarted.current = true;
    (async () => {
      try {
        const res = await fetch('/dashboard/api/numbers/pair', { method: 'POST' });
        // 409 means a number is already paired — checkConnected below will
        // pick that up and show the connected state instead of an error.
        if (!res.ok && res.status !== 409) {
          setError('Failed to start pairing.');
        }
      } catch {
        setError('Failed to start pairing.');
      }
      fetchQr();
      checkConnected();
    })();
  }, [fetchQr, checkConnected]);

  useEffect(() => {
    if (connected) return;
    const interval = setInterval(() => {
      fetchQr();
      checkConnected();
    }, 3000);
    return () => clearInterval(interval);
  }, [connected, fetchQr, checkConnected]);

  return (
    <div className="p-6 max-w-md mx-auto space-y-6">
      <Link
        to="/numbers"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
      >
        <ArrowLeft className="w-4 h-4" />
        Back to Numbers
      </Link>

      <Card className="p-8 flex flex-col items-center gap-4 text-center">
        {connected ? (
          <>
            <CheckCircle2 className="w-12 h-12 text-[#25D366]" />
            <h3 className="text-lg font-semibold">Number linked</h3>
            <p className="text-sm text-muted-foreground">
              Your WhatsApp number is connected and ready.
            </p>
            <Link
              to="/numbers"
              className="mt-2 px-4 py-1.5 rounded-md text-sm font-medium bg-primary text-primary-foreground hover:opacity-90 transition-opacity"
            >
              Back to Numbers
            </Link>
          </>
        ) : (
          <>
            <h3 className="text-lg font-semibold">Scan to link this number</h3>
            <div className="bg-white p-4 rounded-xl shadow-sm border w-64 h-64 flex items-center justify-center">
              {qrCode ? (
                <img src={qrCode} alt="Pairing QR code" className="w-56 h-56 object-contain" />
              ) : (
                <span className="text-xs text-muted-foreground">Waiting for QR code…</span>
              )}
            </div>
            <p className="text-sm text-muted-foreground">
              WhatsApp → Settings → Linked Devices → Link a Device
            </p>
            {error && (
              <div className="flex items-center gap-2 text-destructive text-sm">
                <AlertTriangle className="w-4 h-4 shrink-0" />
                {error}
              </div>
            )}
          </>
        )}
      </Card>
    </div>
  );
}
