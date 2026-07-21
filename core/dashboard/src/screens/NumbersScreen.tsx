import { useCallback, useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { Card } from '../components/ui/card';
import { Smartphone, QrCode, AlertTriangle, Cloud, Settings2 } from 'lucide-react';
import type { CloudStatus } from '../types';

const emptyCloudForm = {
  phoneNumberId: '',
  accessToken: '',
  otpTemplateName: '',
  otpTemplateLanguage: '',
  wabaId: '',
  pin: '',
  appSecret: '',
  verifyToken: '',
};

export default function NumbersScreen() {
  const [numbers, setNumbers] = useState<{ jid: string; phone: string; connected: boolean }[]>([]);
  const [cloud, setCloud] = useState<CloudStatus | null>(null);

  const [showCloudForm, setShowCloudForm] = useState(false);
  const [cloudForm, setCloudForm] = useState(emptyCloudForm);
  const [savingCloud, setSavingCloud] = useState(false);
  const [cloudError, setCloudError] = useState<string | null>(null);

  const fetchNumbers = useCallback(async () => {
    try {
      const res = await fetch('/dashboard/api/numbers');
      if (res.ok) {
        const data: { jid: string; phone: string; connected: boolean }[] = await res.json();
        setNumbers(data);
      }
    } catch {
      // next poll will retry
    }
  }, []);

  const fetchCloudStatus = useCallback(async () => {
    try {
      const res = await fetch('/dashboard/api/cloud-status');
      if (res.ok) setCloud(await res.json());
    } catch {
      // next poll will retry
    }
  }, []);

  useEffect(() => {
    fetchNumbers();
    fetchCloudStatus();
    const interval = setInterval(() => {
      fetchNumbers();
      fetchCloudStatus();
    }, 5000);
    return () => clearInterval(interval);
  }, [fetchNumbers, fetchCloudStatus]);

  const openCloudForm = () => {
    // access_token/pin/app_secret are never sent back by the status
    // endpoint — left blank means "keep the existing one" (see
    // updateCloudSettings), so a form opened to just flip the
    // language/template doesn't force re-entering secrets the operator
    // doesn't have handy. waba_id/verify_token aren't secret and round-trip.
    setCloudForm({
      phoneNumberId: cloud?.phone_number_id ?? '',
      accessToken: '',
      otpTemplateName: cloud?.otp_template_name ?? '',
      otpTemplateLanguage: cloud?.otp_template_language ?? '',
      wabaId: cloud?.waba_id ?? '',
      pin: '',
      appSecret: '',
      verifyToken: cloud?.verify_token ?? '',
    });
    setCloudError(null);
    setShowCloudForm(true);
  };

  const saveCloudSettings = async (enabled: boolean) => {
    setSavingCloud(true);
    setCloudError(null);
    try {
      const res = await fetch('/dashboard/api/cloud-settings', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          enabled,
          phone_number_id: cloudForm.phoneNumberId,
          access_token: cloudForm.accessToken,
          otp_template_name: cloudForm.otpTemplateName,
          otp_template_language: cloudForm.otpTemplateLanguage,
          waba_id: cloudForm.wabaId,
          pin: cloudForm.pin,
          app_secret: cloudForm.appSecret,
          verify_token: cloudForm.verifyToken,
        }),
      });
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

  const waNumber = numbers[0] ?? null;
  const waConnected = !!waNumber?.connected;

  return (
    <div className="p-6 max-w-4xl mx-auto space-y-6">
      <div className="flex flex-col gap-2 mb-2">
        <h2 className="text-3xl font-bold tracking-tight">Numbers</h2>
        <p className="text-muted-foreground text-lg">
          This instance's WhatsApp number and its Cloud API status.
        </p>
      </div>

      <Card className="p-4">
        <div className="flex items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            <div
              className={`p-2 rounded-lg ${
                !waNumber
                  ? 'bg-muted text-muted-foreground'
                  : waConnected
                    ? 'bg-[#25D366]/10 text-[#25D366]'
                    : 'bg-destructive/10 text-destructive'
              }`}
            >
              <Smartphone className="w-4 h-4" />
            </div>
            <div>
              <p className="text-sm font-semibold">WhatsApp (whatsmeow)</p>
              <p className="text-xs text-muted-foreground">
                {waNumber ? waNumber.phone : 'Not paired yet'}
              </p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <div
              className={`flex items-center gap-1.5 text-xs font-semibold px-2 py-1 rounded ${
                !waNumber
                  ? 'bg-muted text-muted-foreground'
                  : waConnected
                    ? 'bg-[#25D366]/10 text-[#25D366]'
                    : 'bg-destructive/10 text-destructive'
              }`}
            >
              {!waNumber ? 'Not paired' : waConnected ? 'Connected' : 'Disconnected'}
            </div>
            {!waNumber && (
              <Link
                to="/numbers/scan"
                className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium bg-muted text-foreground hover:bg-muted/80 transition-colors"
              >
                <QrCode className="w-3.5 h-3.5" />
                Scan
              </Link>
            )}
          </div>
        </div>
      </Card>

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
                    ? 'Not enabled'
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

              <div className="pt-3 border-t space-y-3">
                <p className="text-xs font-semibold text-muted-foreground">
                  Inbound (optional) — receive customer replies on this number
                </p>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <label className="flex flex-col gap-1 text-xs text-muted-foreground">
                    WABA ID
                    <input
                      type="text"
                      value={cloudForm.wabaId}
                      onChange={(e) => setCloudForm((f) => ({ ...f, wabaId: e.target.value }))}
                      className="text-sm bg-muted border rounded-md px-2 py-1.5 text-foreground"
                      placeholder="WhatsApp Business Account ID"
                    />
                  </label>
                  <label className="flex flex-col gap-1 text-xs text-muted-foreground">
                    Two-step verification PIN
                    <input
                      type="password"
                      value={cloudForm.pin}
                      onChange={(e) => setCloudForm((f) => ({ ...f, pin: e.target.value }))}
                      className="text-sm bg-muted border rounded-md px-2 py-1.5 text-foreground"
                      placeholder="Leave blank to skip registering for inbound"
                    />
                  </label>
                  <label className="flex flex-col gap-1 text-xs text-muted-foreground">
                    App secret
                    <input
                      type="password"
                      value={cloudForm.appSecret}
                      onChange={(e) => setCloudForm((f) => ({ ...f, appSecret: e.target.value }))}
                      className="text-sm bg-muted border rounded-md px-2 py-1.5 text-foreground"
                      placeholder="Leave blank to keep the current secret"
                    />
                  </label>
                  <label className="flex flex-col gap-1 text-xs text-muted-foreground">
                    Verify token
                    <input
                      type="text"
                      value={cloudForm.verifyToken}
                      onChange={(e) => setCloudForm((f) => ({ ...f, verifyToken: e.target.value }))}
                      className="text-sm bg-muted border rounded-md px-2 py-1.5 text-foreground"
                      placeholder="Any string — must match what you set in Meta"
                    />
                  </label>
                </div>
                {cloud.webhook_url && (
                  <label className="flex flex-col gap-1 text-xs text-muted-foreground">
                    Webhook URL — paste this into Meta's app dashboard
                    <input
                      type="text"
                      readOnly
                      value={cloud.webhook_url}
                      onFocus={(e) => e.target.select()}
                      className="text-sm bg-muted border rounded-md px-2 py-1.5 text-foreground font-mono"
                    />
                  </label>
                )}
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
    </div>
  );
}
