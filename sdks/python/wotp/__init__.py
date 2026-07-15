"""Wotp — Official Python SDK for WhatsApp OTP, self-hosted.

Usage::

    from wotp import create_client

    client = create_client("http://localhost:54321", "wotp_anon_xxx")
    resp = client.send_otp("+212600000000")
    result = client.verify_otp(resp.token, "483920")
"""

from .client import WotpClient
from .errors import (
    ExpiredTokenError,
    InvalidCodeError,
    RateLimitError,
    WotpError,
)
from .types import HealthResponse, SendOTPResponse, VerifyOTPResponse

__all__ = [
    "create_client",
    "WotpClient",
    "WotpError",
    "RateLimitError",
    "ExpiredTokenError",
    "InvalidCodeError",
    "SendOTPResponse",
    "VerifyOTPResponse",
    "HealthResponse",
]

from importlib.metadata import PackageNotFoundError, version

try:
    __version__ = version("wotp")
except PackageNotFoundError:
    __version__ = "dev"


def create_client(
    url: str,
    api_key: str,
    *,
    max_retries: int = 3,
    retry_delay: float = 0.5,
    timeout: float = 10.0,
) -> WotpClient:
    """Create a new Wotp client instance.

    Args:
        url: Base URL of your Wotp instance (e.g. ``http://localhost:54321``).
        api_key: Your anon or service API key.
        max_retries: Maximum retries on transient network errors (default: 3).
        retry_delay: Base delay in seconds between retries (default: 0.5).
        timeout: Request timeout in seconds (default: 10.0).

    Returns:
        A configured :class:`WotpClient` instance.

    Example::

        from wotp import create_client

        client = create_client("http://localhost:54321", "wotp_anon_xxx")
    """
    return WotpClient(
        url=url,
        api_key=api_key,
        max_retries=max_retries,
        retry_delay=retry_delay,
        timeout=timeout,
    )
