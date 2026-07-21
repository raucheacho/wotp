"""Wotp HTTP client with auto-retry on transient errors."""

from __future__ import annotations

import time
from typing import Any

import httpx

from .errors import (
    ExpiredTokenError,
    InvalidCodeError,
    RateLimitError,
    WotpError,
)
from .types import (
    Chat,
    Conversation,
    ConversationMessage,
    HealthResponse,
    MediaFile,
    MediaKind,
    MessageResponse,
    SendOTPResponse,
    VerifyOTPResponse,
)

_DEFAULT_MAX_RETRIES = 3
_DEFAULT_RETRY_DELAY = 0.5  # seconds
_DEFAULT_TIMEOUT = 10.0  # seconds
_TRANSIENT_STATUS_CODES = {502, 503, 504}


class WotpClient:
    """Official Python client for the Wotp API.

    Usage::

        from wotp import create_client

        client = create_client("http://localhost:54321", "wotp_anon_xxx")
        resp = client.send_otp("+212600000000")
        result = client.verify_otp(resp.token, "483920")

    Args:
        url: Base URL of the Wotp instance.
        api_key: API key for authentication.
        max_retries: Maximum retries on transient network errors.
        retry_delay: Base delay in seconds between retries (exponential backoff).
        timeout: Request timeout in seconds.
    """

    def __init__(
        self,
        url: str,
        api_key: str,
        *,
        max_retries: int = _DEFAULT_MAX_RETRIES,
        retry_delay: float = _DEFAULT_RETRY_DELAY,
        timeout: float = _DEFAULT_TIMEOUT,
    ) -> None:
        self._base_url = url.rstrip("/")
        self._api_key = api_key
        self._max_retries = max_retries
        self._retry_delay = retry_delay
        self._client = httpx.Client(
            base_url=self._base_url,
            timeout=timeout,
            headers={
                "Content-Type": "application/json",
                "apikey": self._api_key,
            },
        )

    def __enter__(self) -> WotpClient:
        return self

    def __exit__(self, *args: Any) -> None:
        self.close()

    def close(self) -> None:
        """Close the underlying HTTP client."""
        self._client.close()

    # ─── Public API ──────────────────────────────────────────────

    def send_otp(self, phone: str) -> SendOTPResponse:
        """Send an OTP to the given phone number.

        Args:
            phone: E.164 formatted phone number (e.g. ``+212600000000``).

        Returns:
            Token and expiration timestamp.

        Raises:
            RateLimitError: If the phone/IP has exceeded the rate limit.
        """
        data = self._request("POST", "/v1/otp/send", json={"phone": phone})
        return SendOTPResponse.model_validate(data)

    def verify_otp(self, token: str, code: str) -> VerifyOTPResponse:
        """Verify an OTP code against a previously issued token.

        Args:
            token: The opaque token returned by :meth:`send_otp`.
            code: The OTP code entered by the user.

        Returns:
            Verification result.

        Raises:
            ExpiredTokenError: If the token has expired.
            InvalidCodeError: If the code is incorrect.
        """
        data = self._request("POST", "/v1/otp/verify", json={"token": token, "code": code})
        return VerifyOTPResponse.model_validate(data)

    def health(self) -> HealthResponse:
        """Check the health of the Wotp instance.

        Returns:
            Connection status, phone number, and uptime.
        """
        data = self._request("GET", "/v1/health")
        return HealthResponse.model_validate(data)


    def send_text(self, phone: str, text: str) -> MessageResponse:
        """Send a text message."""
        data = self._request("POST", "/v1/messages/send", json={"phone": phone, "type": "text", "text": text})
        return MessageResponse.model_validate(data)

    def send_media(
        self,
        phone: str,
        url: str | None = None,
        base64: str | None = None,
        caption: str | None = None,
        kind: MediaKind = "image",
        filename: str | None = None,
    ) -> MessageResponse:
        """Send a media message (image, video, audio, or document).

        Args:
            phone: E.164 formatted phone number.
            url: A publicly reachable URL wotp fetches the file from.
            base64: The file's bytes, base64-encoded (alternative to ``url``).
            caption: Optional caption (image/video/document only).
            kind: One of ``"image"``, ``"video"``, ``"audio"``, ``"document"``.
            filename: Shown as the file name in the recipient's chat. Only
                meaningful when ``kind`` is ``"document"``.
        """
        payload: dict[str, Any] = {"phone": phone, "type": kind}
        if url: payload["url"] = url
        if base64: payload["base64"] = base64
        if caption: payload["caption"] = caption
        if filename: payload["filename"] = filename
        data = self._request("POST", "/v1/messages/send", json=payload)
        return MessageResponse.model_validate(data)

    def send_location(
        self,
        phone: str,
        latitude: float,
        longitude: float,
        name: str | None = None,
        address: str | None = None,
    ) -> MessageResponse:
        """Send a WhatsApp location message.

        Args:
            phone: E.164 formatted phone number.
            latitude: Required.
            longitude: Required.
            name: Optional place name label.
            address: Optional address label.
        """
        payload: dict[str, Any] = {
            "phone": phone,
            "type": "location",
            "latitude": latitude,
            "longitude": longitude,
        }
        if name: payload["name"] = name
        if address: payload["address"] = address
        data = self._request("POST", "/v1/messages/send", json=payload)
        return MessageResponse.model_validate(data)

    def get_chats(self) -> list[Chat]:
        """List the WhatsApp contacts visible to the connected number."""
        data = self._request("GET", "/v1/chats")
        return [Chat.model_validate(c) for c in data]

    def set_presence(self, phone: str, state: str) -> None:
        """Set the typing indicator for a chat without sending a message.

        Args:
            phone: E.164 formatted phone number.
            state: ``"typing"`` or ``"paused"``.
        """
        self._request("POST", "/v1/messages/presence", json={"phone": phone, "state": state})

    # ─── Conversations & takeover ───────────────────────────────────

    def list_conversations(self) -> list[Conversation]:
        """List every tracked conversation (one per contact that has ever messaged in)."""
        data = self._request("GET", "/v1/conversations")
        return [Conversation.model_validate(c) for c in data]

    def get_conversation(self, conversation_id: str) -> Conversation:
        """Fetch a single conversation by id."""
        data = self._request("GET", f"/v1/conversations/{conversation_id}")
        return Conversation.model_validate(data)

    def get_conversation_messages(self, conversation_id: str) -> list[ConversationMessage]:
        """Get the full chronological thread for a conversation — inbound
        replies, outbound sends, and OTP sends merged together."""
        data = self._request("GET", f"/v1/conversations/{conversation_id}/messages")
        return [ConversationMessage.model_validate(m) for m in data]

    def takeover_conversation(
        self, conversation_id: str, actor: str | None = None, reason: str | None = None
    ) -> None:
        """Mark a conversation as human-owned. wotp keeps forwarding
        ``message.received`` either way — it's up to your own bot logic to
        read ``conversation_state`` from the webhook payload and stay quiet.
        """
        self._set_conversation_state(conversation_id, "takeover", actor, reason)

    def resume_conversation(
        self, conversation_id: str, actor: str | None = None, reason: str | None = None
    ) -> None:
        """Hand a conversation back to the bot."""
        self._set_conversation_state(conversation_id, "resume", actor, reason)

    def _set_conversation_state(
        self, conversation_id: str, action: str, actor: str | None, reason: str | None
    ) -> None:
        payload: dict[str, Any] = {}
        if actor: payload["actor"] = actor
        if reason: payload["reason"] = reason
        self._request("POST", f"/v1/conversations/{conversation_id}/{action}", json=payload)

    # ─── Inbound media ───────────────────────────────────────────────

    def get_media(self, message_id: str) -> MediaFile:
        """Download the raw bytes of an inbound media message wotp captured
        at receive time (see :class:`ConversationMessage`.kind /
        :data:`MediaKind`). Raises :class:`~wotp.errors.WotpError` with
        ``status_code`` 404 if the message wasn't a media message, or if the
        download itself failed when the message arrived.
        """
        try:
            response = self._client.request("GET", f"/v1/media/{message_id}")
        except httpx.HTTPError as exc:
            raise WotpError(f"Network error: {exc}") from exc

        if response.is_success:
            return MediaFile(
                data=response.content,
                content_type=response.headers.get("Content-Type", ""),
            )

        try:
            error_body = response.json()
        except Exception:
            error_body = {}
        msg = error_body.get("message", response.text)
        raise WotpError(f"Request failed ({response.status_code}): {msg}", status_code=response.status_code)

    # ─── Internal ────────────────────────────────────────────────

    def _request(
        self,
        method: str,
        path: str,
        *,
        json: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        """Make an HTTP request with auto-retry on transient errors.

        Business errors (4xx) are never retried.
        """
        last_error: Exception | None = None

        for attempt in range(self._max_retries + 1):
            try:
                response = self._client.request(method, path, json=json)
            except httpx.HTTPError as exc:
                # Network / timeout errors — retry
                last_error = WotpError(f"Network error: {exc}")
                if attempt < self._max_retries:
                    self._sleep(attempt)
                    continue
                raise last_error from exc

            # Success
            if response.is_success:
                return response.json()  # type: ignore[no-any-return]

            # Parse error body
            try:
                error_body = response.json()
            except Exception:
                error_body = {}

            # Business errors — never retry
            if response.status_code == 429:
                retry_after_header = response.headers.get("Retry-After")
                retry_after = int(retry_after_header) if retry_after_header else None
                raise RateLimitError(
                    message=error_body.get("message", "Rate limit exceeded"),
                    retry_after=retry_after,
                )

            if response.status_code in (400, 410):
                error_type = error_body.get("error", "")

                if error_type == "token_expired" or response.status_code == 410:
                    raise ExpiredTokenError(
                        message=error_body.get("message", "OTP token has expired"),
                    )

                if error_type == "invalid_code":
                    raise InvalidCodeError(
                        message=error_body.get("message", "Invalid OTP code"),
                        attempts_remaining=error_body.get("attempts_remaining"),
                    )

            # Transient server errors — retry
            if response.status_code in _TRANSIENT_STATUS_CODES:
                last_error = WotpError(
                    f"Server error {response.status_code}: {response.text}",
                    status_code=response.status_code,
                )
                if attempt < self._max_retries:
                    self._sleep(attempt)
                    continue
                raise last_error

            # Unknown error
            msg = error_body.get("message", response.text)
            raise WotpError(f"Request failed ({response.status_code}): {msg}", status_code=response.status_code)

        if last_error is not None:
            raise last_error
        raise WotpError("Request failed after retries")  # pragma: no cover

    def _sleep(self, attempt: int) -> None:
        """Sleep with exponential backoff."""
        delay = self._retry_delay * (2**attempt)
        time.sleep(delay)
