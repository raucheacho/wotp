/**
 * Format a phone number with partial masking for privacy.
 * +212600000000 → +212 6XX XX 00 00
 */
export function maskPhone(phone: string): string {
  if (!phone) return '—';
  const cleaned = phone.replace(/\s/g, '');
  if (cleaned.length >= 12) {
    return `${cleaned.slice(0, 4)} ${cleaned.slice(4, 5)}XX XX ${cleaned.slice(-4, -2)} ${cleaned.slice(-2)}`;
  }
  if (cleaned.length >= 10) {
    return `${cleaned.slice(0, 3)} ${'•'.repeat(cleaned.length - 5)}${cleaned.slice(-2)}`;
  }
  return phone;
}

/**
 * Relative timestamp (e.g. "2m ago", "1h ago", "just now")
 */
export function timeAgo(dateStr: string): string {
  try {
    const now = Date.now();
    const then = new Date(dateStr).getTime();
    const diff = Math.max(0, now - then);

    const seconds = Math.floor(diff / 1000);
    if (seconds < 5) return 'just now';
    if (seconds < 60) return `${seconds}s ago`;

    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes}m ago`;

    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours}h ago`;

    const days = Math.floor(hours / 24);
    return `${days}d ago`;
  } catch {
    return '—';
  }
}

/**
 * Format a timestamp for logs (HH:MM:SS.mmm)
 */
export function formatLogTime(dateStr: string): string {
  try {
    const d = new Date(dateStr);
    return d.toLocaleTimeString('en-US', {
      hour12: false,
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    }) + '.' + String(d.getMilliseconds()).padStart(3, '0');
  } catch {
    return '00:00:00.000';
  }
}

/**
 * Copy text to clipboard
 */
export async function copyToClipboard(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    // Fallback
    const textarea = document.createElement('textarea');
    textarea.value = text;
    textarea.style.position = 'fixed';
    textarea.style.opacity = '0';
    document.body.appendChild(textarea);
    textarea.select();
    try {
      document.execCommand('copy');
      document.body.removeChild(textarea);
      return true;
    } catch {
      document.body.removeChild(textarea);
      return false;
    }
  }
}

/**
 * Format uptime from seconds
 */
export function formatUptime(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  const mins = minutes % 60;
  if (hours < 24) return `${hours}h ${mins}m`;
  const days = Math.floor(hours / 24);
  return `${days}d ${hours % 24}h`;
}
