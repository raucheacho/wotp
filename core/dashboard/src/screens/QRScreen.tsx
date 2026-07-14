import { useEffect, useState } from 'react';
import { useStore } from '../store';

export default function QRScreen() {
  const connectionStatus = useStore(state => state.connectionStatus);
  const [qrUrl, setQrUrl] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    const fetchQR = async () => {
      try {
        setLoading(true);
        setError(null);
        const response = await fetch('/qr');
        if (!response.ok) {
          throw new Error(`Failed to fetch QR code (${response.status})`);
        }
        const blob = await response.blob();
        if (!cancelled) {
          const url = URL.createObjectURL(blob);
          setQrUrl(url);
          setLoading(false);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Failed to load QR code');
          setLoading(false);
        }
      }
    };

    fetchQR();

    // Refresh QR every 30s in case it expires
    const interval = setInterval(fetchQR, 30000);

    return () => {
      cancelled = true;
      clearInterval(interval);
      if (qrUrl) URL.revokeObjectURL(qrUrl);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const connectionLabel =
    connectionStatus === 'connected'
      ? 'Connected!'
      : connectionStatus === 'connecting'
      ? 'Connecting...'
      : 'Waiting for scan...';

  return (
    <div className="qr-screen">
      <div className="qr-container">
        {/* Header */}
        <div style={{ marginBottom: 8 }}>
          <h2 style={{ fontSize: '1.4rem', fontWeight: 700, letterSpacing: '-0.5px' }}>
            Connect WhatsApp
          </h2>
          <p style={{ color: 'var(--text-secondary)', fontSize: '0.85rem', marginTop: 6 }}>
            Scan the QR code below with your phone to link this device
          </p>
        </div>

        {/* QR Image */}
        <div className="qr-image-wrapper">
          {loading ? (
            <div
              style={{
                width: 240,
                height: 240,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                background: '#f0f0f0',
              }}
            >
              <div className="spinner spinner-lg" style={{ borderTopColor: '#25D366' }} />
            </div>
          ) : error ? (
            <div
              style={{
                width: 240,
                height: 240,
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                justifyContent: 'center',
                gap: 12,
                background: '#f9f9f9',
              }}
            >
              <span style={{ fontSize: '2rem' }}>⚠️</span>
              <span style={{ fontSize: '0.75rem', color: '#666', textAlign: 'center', padding: '0 16px' }}>
                {error}
              </span>
              <button
                className="btn btn-secondary btn-sm"
                onClick={() => window.location.reload()}
              >
                Retry
              </button>
            </div>
          ) : (
            <>
              <img src={qrUrl || ''} alt="WhatsApp QR Code" />
              <div className="qr-shimmer" />
            </>
          )}
        </div>

        {/* Status */}
        <div className="qr-status">
          {connectionStatus === 'connected' ? (
            <>
              <div
                className="status-dot connected"
                style={{ width: 8, height: 8, borderRadius: '50%', background: 'var(--accent)' }}
              />
              <span style={{ color: 'var(--accent)', fontWeight: 600 }}>{connectionLabel}</span>
            </>
          ) : (
            <>
              <div className="pulse" />
              <span>{connectionLabel}</span>
            </>
          )}
        </div>

        {/* Instructions */}
        <div className="qr-instructions">
          <ol>
            <li>Open <strong style={{ color: 'var(--text-primary)' }}>WhatsApp</strong> on your phone</li>
            <li>Go to <strong style={{ color: 'var(--text-primary)' }}>Settings → Linked Devices</strong></li>
            <li>Tap <strong style={{ color: 'var(--text-primary)' }}>Link a Device</strong></li>
            <li>Point your phone at this <strong style={{ color: 'var(--text-primary)' }}>QR code</strong></li>
          </ol>
        </div>
      </div>

      {/* Branding */}
      <div style={{ color: 'var(--text-tertiary)', fontSize: '0.75rem', fontFamily: 'var(--font-mono)' }}>
        wotp v1.0.0
      </div>
    </div>
  );
}
