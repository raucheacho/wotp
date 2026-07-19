import { useCallback, useEffect, useRef, useState } from 'react';
import { useStore } from '../store';
import { Card } from '../components/ui/card';
import { Smartphone, Plus, AlertTriangle, Cloud, Settings2, X } from 'lucide-react';
import type { CloudStatus } from '../types';

const emptyCloudForm = {
  phoneNumberId: '',
  accessToken: '',
  otpTemplateName: '',
  otpTemplateLanguage: '',
};

export default function NumbersScreen() {
  const selectedProjectId = useStore((state) => state.selectedProjectId);
  const [numbers, setNumbers] = useState<{ jid: string; phone: string; connected: boolean }[]>([]);
  const [cloud, setCloud] = useState<CloudStatus | null>(null);
  const [qrCode, setQrCode] = useState<string | null>(null);
  const [starting, setStarting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Ref, not state: read inside the fetchQr poll without needing to
  // recreate that callback (and its setInterval) every time it flips.
  const qrDismissed = useRef(false);

  const [showCloudForm, setShowCloudForm] = useState(false);
  const [cloudForm, setCloudForm] = useState(emptyCloudForm);
  const [savingCloud, setSavingCloud] = useState(false);
  const [cloudError, setCloudError] = useState<string | null>(null);

  const fetchNumbers = useCallback(async () => {
    if (!selectedProjectId) return;
    try {
      const res = await fetch(`/dashboard/api/numbers?project_id=${encodeURIComponent(selectedProjectId)}`);
      if (res.ok) {
        const data: { jid: string; phone: string; connected: boolean }[] = await res.json();
        setNumbers(data);
        // Don't wait for the next QR poll's 503 to hide a stale code once a
        // number is actually connected — the backend clears it once pairing
        // ends, but this reacts a poll cycle sooner.
        if (data.some((n) => n.connected)) setQrCode(null);
      }
    } catch {
      // next poll will retry
    }
  }, [selectedProjectId]);

  const fetchCloudStatus = useCallback(async () => {
    if (!selectedProjectId) return;
    try {
      const res = await fetch(
        `/dashboard/api/cloud-status?project_id=${encodeURIComponent(selectedProjectId)}`,
      );
      if (res.ok) setCloud(await res.json());
    } catch {
      // next poll will retry
    }
  }, [selectedProjectId]);

  // Polls for a QR regardless of whether the project already has connected
  // numbers — unlike the initial-setup QRScreen (gated on the whole project
  // being disconnected), this is how you see the code for a pairing
  // triggered while other numbers are already up and running (e.g. via
  // `wotp project add-number`, or the "Add number" button below).
  const fetchQr = useCallback(async () => {
    if (!selectedProjectId) return;
    try {
      const res = await fetch(
        `/dashboard/api/numbers/qr?project_id=${encodeURIComponent(selectedProjectId)}`,
        { headers: { Accept: 'application/json' } },
      );
      if (res.status === 200) {
        const data = await res.json();
        if (data.qr && !qrDismissed.current) {
          const hash = btoa(data.qr).replace(/=/g, '').substring(0, 20);
          setQrCode(
            `/dashboard/api/numbers/qr?project_id=${encodeURIComponent(selectedProjectId)}&hash=${hash}`,
          );
        }
      } else {
        // 503 = no pairing in progress right now — not an error.
        setQrCode(null);
      }
    } catch {
      // next poll will retry
    }
  }, [selectedProjectId]);

  useEffect(() => {
    fetchNumbers();
    fetchQr();
    fetchCloudStatus();
    const interval = setInterval(() => {
      fetchNumbers();
      fetchQr();
      fetchCloudStatus();
    }, 5000);
    return () => clearInterval(interval);
  }, [fetchNumbers, fetchQr, fetchCloudStatus]);

  const startPairing = async () => {
    if (!selectedProjectId) return;
    setStarting(true);
    setError(null);
    qrDismissed.current = false;
    try {
      const res = await fetch(
        `/dashboard/api/numbers/pair?project_id=${encodeURIComponent(selectedProjectId)}`,
        { method: 'POST' },
      );
      if (!res.ok) setError('Failed to start pairing.');
      else fetchQr();
    } catch {
      setError('Failed to start pairing.');
    } finally {
      setStarting(false);
    }
  };

  const openCloudForm = () => {
    // access_token is never sent back by the status endpoint — left blank
    // means "keep the existing one" (see updateCloudSettings), so a form
    // opened to just flip the language/template doesn't force re-entering
    // a secret the operator doesn't have handy.
    setCloudForm({
      phoneNumberId: cloud?.phone_number_id ?? '',
      accessToken: '',
      otpTemplateName: cloud?.otp_template_name ?? '',
      otpTemplateLanguage: cloud?.otp_template_language ?? '',
    });
    setCloudError(null);
    setShowCloudForm(true);
  };

  const saveCloudSettings = async (enabled: boolean) => {
    if (!selectedProjectId) return;
    setSavingCloud(true);
    setCloudError(null);
    try {
      const res = await fetch(
        `/dashboard/api/cloud-settings?project_id=${encodeURIComponent(selectedProjectId)}`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            enabled,
            phone_number_id: cloudForm.phoneNumberId,
            access_token: cloudForm.accessToken,
            otp_template_name: cloudForm.otpTemplateName,
            otp_template_language: cloudForm.otpTemplateLanguage,
          }),
        },
      );
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        setCloudError(body.error || 'Failed to save Cloud API settings.');
        return;
      }
      setShowCloudForm(false);
      fetchCloudStatus();
    } catch {
      setCloudError('Failed to save Cloud API settings.');
    } finally {
      setSavingCloud(false);
    }
  };

  return (
    <div className="p-6 max-w-4xl mx-auto space-y-6">
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 mb-2">
        <div className="flex flex-col gap-2">
          <h2 className="text-3xl font-bold tracking-tight">Numbers</h2>
          <p className="text-muted-foreground text-lg">
            This project's WhatsApp number and its Cloud API status.
          </p>
        </div>
        <button
          onClick={startPairing}
          disabled={starting || numbers.length >= 1}
          title={numbers.length >= 1 ? 'This project already has a number — each project is limited to one' : undefined}
          className="flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground rounded-md font-medium text-sm hover:opacity-90 disabled:opacity-50 transition-opacity"
        >
          <Plus className="w-4 h-4" />
          {starting ? 'Starting…' : 'Add number'}
        </button>
      </div>

      {cloud && (
        <Card className="p-4">
          <div className="flex items-center justify-between gap-4">
            <div className="flex items-center gap-3">
              <div
                className={`p-2 rounded-lg ${
                  !cloud.enabled
                    ? 'bg-muted text-muted-foreground'
                    : cloud.connected
                      ? 'bg-[#25D366]/10 text-[#25D366]'
                      : 'bg-destructive/10 text-destructive'
                }`}
              >
                <Cloud className="w-4 h-4" />
              </div>
              <div>
                <p className="text-sm font-semibold">Cloud API (OTP)</p>
                <p className="text-xs text-muted-foreground">
                  {!cloud.enabled
                    ? 'Not enabled for this project'
                    : cloud.display_phone || cloud.phone_number_id}
                </p>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <div
                className={`flex items-center gap-1.5 text-xs font-semibold px-2 py-1 rounded ${
                  !cloud.enabled
                    ? 'bg-muted text-muted-foreground'
                    : cloud.connected
                      ? 'bg-[#25D366]/10 text-[#25D366]'
                      : 'bg-destructive/10 text-destructive'
                }`}
              >
                {!cloud.enabled ? 'Disabled' : cloud.connected ? 'Connected' : 'Not verified'}
              </div>
              {!showCloudForm && (
                <button
                  onClick={openCloudForm}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium bg-muted text-foreground hover:bg-muted/80 transition-colors"
                >
                  <Settings2 className="w-3.5 h-3.5" />
                  {cloud.enabled ? 'Edit' : 'Configure'}
                </button>
              )}
            </div>
          </div>

          {showCloudForm && (
            <div className="mt-4 pt-4 border-t space-y-3">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <label className="flex flex-col gap-1 text-xs text-muted-foreground">
                  Phone number ID
                  <input
                    type="text"
                    value={cloudForm.phoneNumberId}
                    onChange={(e) => setCloudForm((f) => ({ ...f, phoneNumberId: e.target.value }))}
                    className="text-sm bg-muted border rounded-md px-2 py-1.5 text-foreground"
                    placeholder="from Meta's WhatsApp API Setup"
                  />
                </label>
                <label className="flex flex-col gap-1 text-xs text-muted-foreground">
                  Access token
                  <input
                    type="password"
                    value={cloudForm.accessToken}
                    onChange={(e) => setCloudForm((f) => ({ ...f, accessToken: e.target.value }))}
                    className="text-sm bg-muted border rounded-md px-2 py-1.5 text-foreground"
                    placeholder={cloud.enabled ? 'Leave blank to keep the current token' : ''}
                  />
                </label>
                <label className="flex flex-col gap-1 text-xs text-muted-foreground">
                  OTP template name
                  <input
                    type="text"
                    value={cloudForm.otpTemplateName}
                    onChange={(e) => setCloudForm((f) => ({ ...f, otpTemplateName: e.target.value }))}
                    className="text-sm bg-muted border rounded-md px-2 py-1.5 text-foreground"
                    placeholder="otp_verification"
                  />
                </label>
                <label className="flex flex-col gap-1 text-xs text-muted-foreground">
                  Template language
                  <input
                    type="text"
                    value={cloudForm.otpTemplateLanguage}
                    onChange={(e) => setCloudForm((f) => ({ ...f, otpTemplateLanguage: e.target.value }))}
                    className="text-sm bg-muted border rounded-md px-2 py-1.5 text-foreground"
                    placeholder="en_US"
                  />
                </label>
              </div>

              {cloudError && (
                <div className="flex items-center gap-2 text-destructive text-sm">
                  <AlertTriangle className="w-4 h-4 shrink-0" />
                  {cloudError}
                </div>
              )}

              <div className="flex items-center gap-2">
                <button
                  onClick={() => saveCloudSettings(true)}
                  disabled={savingCloud || !cloudForm.phoneNumberId}
                  className="px-4 py-1.5 rounded-md text-sm font-medium bg-primary text-primary-foreground hover:opacity-90 disabled:opacity-50 transition-opacity"
                >
                  {savingCloud ? 'Saving…' : 'Save & enable'}
                </button>
                {cloud.enabled && (
                  <button
                    onClick={() => saveCloudSettings(false)}
                    disabled={savingCloud}
                    className="px-4 py-1.5 rounded-md text-sm font-medium bg-destructive/10 text-destructive hover:bg-destructive/20 disabled:opacity-50 transition-colors"
                  >
                    Disable
                  </button>
                )}
                <button
                  onClick={() => setShowCloudForm(false)}
                  disabled={savingCloud}
                  className="px-4 py-1.5 rounded-md text-sm font-medium text-muted-foreground hover:bg-muted transition-colors"
                >
                  Cancel
                </button>
              </div>
            </div>
          )}
        </Card>
      )}

      <Card className="p-4">
        {numbers.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-12 text-muted-foreground text-center">
            <Smartphone className="w-12 h-12 mb-4 opacity-50" />
            <h3 className="text-lg font-semibold text-foreground mb-2">No whatsmeow number yet</h3>
            <p className="max-w-sm">
              Click "Add number" to pair one by QR — or skip whatsmeow entirely and configure the
              Cloud API backend above.
            </p>
          </div>
        ) : (
          <div className="space-y-2">
            {numbers.map((n) => (
              <div
                key={n.jid}
                className="flex items-center justify-between p-4 bg-muted/30 rounded-lg gap-4"
              >
                <div className="flex items-center gap-3">
                  <Smartphone className="w-4 h-4 text-muted-foreground" />
                  <span className="font-mono text-sm">{n.phone}</span>
                </div>
                <div
                  className={`flex items-center gap-1.5 text-xs font-semibold px-2 py-1 rounded ${
                    n.connected
                      ? 'bg-[#25D366]/10 text-[#25D366]'
                      : 'bg-destructive/10 text-destructive'
                  }`}
                >
                  {n.connected ? 'Connected' : 'Disconnected'}
                </div>
              </div>
            ))}
          </div>
        )}
      </Card>

      {error && (
        <div className="flex items-center gap-2 text-destructive text-sm">
          <AlertTriangle className="w-4 h-4" />
          {error}
        </div>
      )}

      {qrCode && (
        <Card className="p-8 flex flex-col items-center gap-4 relative">
          <button
            onClick={() => {
              qrDismissed.current = true;
              setQrCode(null);
            }}
            aria-label="Close"
            title="Close — this does not cancel pairing on the server, it just hides the code. Click Add number again to bring it back."
            className="absolute top-3 right-3 p-1.5 rounded-md text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
          >
            <X className="w-4 h-4" />
          </button>
          <h3 className="text-lg font-semibold">Scan to link this number</h3>
          <div className="bg-white p-4 rounded-xl shadow-sm border">
            <img src={qrCode} alt="Pairing QR code" className="w-56 h-56 object-contain" />
          </div>
          <p className="text-sm text-muted-foreground">
            WhatsApp → Settings → Linked Devices → Link a Device
          </p>
        </Card>
      )}
    </div>
  );
}
