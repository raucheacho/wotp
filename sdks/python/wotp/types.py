"""Pydantic models for Wotp API responses."""

from __future__ import annotations

from datetime import datetime

from pydantic import BaseModel, Field


class SendOTPResponse(BaseModel):
    """Response from POST /otp/send."""

    token: str = Field(description="Opaque token to use for verification.")
    expires_at: datetime = Field(
        alias="expires_at",
        description="Timestamp when this OTP expires.",
    )
    warning: str | None = Field(
        default=None,
        description=(
            'Set to "message_send_failed" when the OTP was created but the '
            "WhatsApp send itself failed (e.g. no number is connected yet). "
            "The token is still valid — only delivery failed."
        ),
    )

    model_config = {"populate_by_name": True}


class VerifyOTPResponse(BaseModel):
    """Response from POST /otp/verify."""

    verified: bool = Field(description="Whether the OTP code was correct.")
    phone: str | None = Field(
        default=None,
        description="Verified phone number (only when verified is True).",
    )
    attempts_remaining: int | None = Field(
        default=None,
        alias="attempts_remaining",
        description="Remaining attempts (only when verified is False).",
    )

    model_config = {"populate_by_name": True}


class HealthResponse(BaseModel):
    """Response from GET /v1/health.

    This is an instance-wide liveness check — it has no notion of a single
    connected phone number, since one instance can host many projects each
    with their own numbers. See :meth:`WotpClient.get_chats` or the
    dashboard for per-project connection state.
    """

    status: str = Field(description='"ok" when the instance is up.')
    uptime_seconds: int = Field(
        alias="uptime_seconds",
        description="Uptime in seconds.",
    )

    model_config = {"populate_by_name": True}


class MessageResponse(BaseModel):
    """Response from POST /v1/messages/send.

    There is no ``success`` field — a failed send comes back as a non-2xx
    status and raises a :class:`~wotp.errors.WotpError` instead.
    """

    message_id: str | None = Field(default=None, alias="message_id")

    model_config = {"populate_by_name": True}


class Chat(BaseModel):
    """A WhatsApp contact visible to one of the project's connected numbers."""

    jid: str
    name: str | None = None

    model_config = {"populate_by_name": True}
