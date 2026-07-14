import { useCallback, useState } from 'react';
import { useStore } from '../store';
import { copyToClipboard } from '../utils';

interface ModalProps {
  title: string;
  message: string;
  confirmLabel: string;
  danger?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

function ConfirmModal({ title, message, confirmLabel, danger, onConfirm, onCancel }: ModalProps) {
  return (
    <div className="modal-overlay" onClick={onCancel}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h3>{title}</h3>
        <p>{message}</p>
        <div className="modal-actions">
          <button className="btn btn-secondary" onClick={onCancel}>
            Cancel
          </button>
          <button
            className={`btn ${danger ? 'btn-danger' : 'btn-primary'}`}
            onClick={onConfirm}
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}

// Simulated config & keys
const MOCK_CONFIG = {
  project: { name: 'livo-otp', ref: 'livo-otp-a8f3' },
  api: { port: 54321, enable_dashboard: true },
  otp: { code_length: 6, expiry_minutes: 5, max_attempts: 5, rate_limit_per_phone_per_hour: 3 },
  whatsapp: { device_name: 'Wotp - livo-otp', reconnect_backoff_seconds: [5, 15, 60, 300] },
  storage: { driver: 'sqlite' },
  templates: { default_locale: 'fr' },
};

const MOCK_KEYS = {
  anon: 'wotp_anon_sk_a8f3e7d6c5b4a392f1e0d9c8b7a6f5e4',
  service: 'wotp_service_sk_9f8e7d6c5b4a392f1e0d9c8b7a6f5e4d',
};

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(async () => {
    const ok = await copyToClipboard(text);
    if (ok) {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  }, [text]);

  return (
    <button
      className={`copy-btn ${copied ? 'copied' : ''}`}
      onClick={handleCopy}
      title="Copy to clipboard"
    >
      {copied ? '✓' : '📋'}
    </button>
  );
}

export default function SettingsScreen() {
  const theme = useStore(state => state.theme);
  const setTheme = useStore(state => state.setTheme);
  const [modal, setModal] = useState<'regenerate' | 'disconnect' | null>(null);
  const [toast, setToast] = useState<string | null>(null);

  const showToast = (msg: string) => {
    setToast(msg);
    setTimeout(() => setToast(null), 2000);
  };

  const handleRegenerate = () => {
    setModal(null);
    showToast('API keys regenerated successfully');
  };

  const handleDisconnect = () => {
    setModal(null);
    showToast('WhatsApp session disconnected');
  };

  return (
    <div className="page-container">
      <div className="page-header">
        <h2>Settings</h2>
        <p>API keys, configuration, and session management</p>
      </div>

      <div className="settings-grid">
        {/* Appearance */}
        <div className="settings-section">
          <div className="settings-section-header">
            <h3>🎨 Appearance</h3>
          </div>
          <div className="settings-section-body">
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span className="api-key-label">Theme</span>
              <select 
                className="input" 
                style={{ width: '150px' }}
                value={theme} 
                onChange={(e) => setTheme(e.target.value as any)}
              >
                <option value="dark">Dark Mode</option>
                <option value="light">Light Mode</option>
              </select>
            </div>
          </div>
        </div>

        {/* API Keys */}
        <div className="settings-section">
          <div className="settings-section-header">
            <h3>🔑 API Keys</h3>
            <button
              className="btn btn-ghost btn-sm"
              onClick={() => setModal('regenerate')}
            >
              Regenerate
            </button>
          </div>
          <div className="settings-section-body">
            <div className="api-key-row">
              <span className="api-key-label">Anon</span>
              <span className="api-key-value">{MOCK_KEYS.anon}</span>
              <CopyButton text={MOCK_KEYS.anon} />
            </div>
            <div className="api-key-row">
              <span className="api-key-label">Service</span>
              <span className="api-key-value">{MOCK_KEYS.service}</span>
              <CopyButton text={MOCK_KEYS.service} />
            </div>
            <div
              className="note-banner"
              style={{ marginTop: 16 }}
            >
              <span className="note-icon">ℹ️</span>
              <span>
                The <strong>anon</strong> key is for client-side OTP send/verify.
                The <strong>service</strong> key grants admin access (regenerate keys, disconnect session).
                Never expose the service key publicly.
              </span>
            </div>
          </div>
        </div>

        {/* Configuration */}
        <div className="settings-section">
          <div className="settings-section-header">
            <h3>⚙️ Configuration</h3>
            <span
              className="badge badge-info"
              style={{ fontSize: '0.65rem' }}
            >
              READ ONLY
            </span>
          </div>
          <div className="settings-section-body">
            <div className="config-display">
              {Object.entries(MOCK_CONFIG).map(([section, values]) => (
                <div key={section} style={{ marginBottom: 12 }}>
                  <span style={{ color: 'var(--text-tertiary)' }}>[{section}]</span>
                  {'\n'}
                  {Object.entries(values as Record<string, unknown>).map(([key, value]) => (
                    <span key={key}>
                      <span className="config-key">{key}</span>
                      {' = '}
                      <span className="config-value">
                        {typeof value === 'object' ? JSON.stringify(value) : String(value)}
                      </span>
                      {'\n'}
                    </span>
                  ))}
                </div>
              ))}
            </div>
            <div className="note-banner" style={{ marginTop: 8 }}>
              <span className="note-icon">📝</span>
              <span>
                Edit <code style={{ fontFamily: 'var(--font-mono)', fontWeight: 600 }}>config.toml</code> and
                run <code style={{ fontFamily: 'var(--font-mono)', fontWeight: 600 }}>wotp restart</code> to
                apply changes.
              </span>
            </div>
          </div>
        </div>

        {/* Danger Zone */}
        <div className="settings-section danger-zone">
          <div className="settings-section-header">
            <h3>⚠️ Danger Zone</h3>
          </div>
          <div className="settings-section-body">
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                gap: 16,
              }}
            >
              <div>
                <div style={{ fontWeight: 600, fontSize: '0.9rem', marginBottom: 4 }}>
                  Disconnect WhatsApp
                </div>
                <div style={{ color: 'var(--text-secondary)', fontSize: '0.82rem' }}>
                  This will unlink the WhatsApp session and require a new QR code scan.
                </div>
              </div>
              <button
                className="btn btn-danger"
                onClick={() => setModal('disconnect')}
                style={{ flexShrink: 0 }}
              >
                Disconnect
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Modals */}
      {modal === 'regenerate' && (
        <ConfirmModal
          title="Regenerate API Keys"
          message="This will invalidate all existing API keys immediately. Any applications using the current keys will need to be updated. This action cannot be undone."
          confirmLabel="Regenerate Keys"
          onConfirm={handleRegenerate}
          onCancel={() => setModal(null)}
        />
      )}
      {modal === 'disconnect' && (
        <ConfirmModal
          title="Disconnect WhatsApp"
          message="This will terminate the current WhatsApp session. You'll need to scan a new QR code to reconnect. Any pending OTP deliveries may fail."
          confirmLabel="Disconnect"
          danger
          onConfirm={handleDisconnect}
          onCancel={() => setModal(null)}
        />
      )}

      {/* Toast */}
      {toast && <div className="tooltip-copied">{toast}</div>}
    </div>
  );
}
