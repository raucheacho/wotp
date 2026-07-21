"""Pydantic models for Wotp API responses."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from typing import Literal

from pydantic import BaseModel, Field

MediaKind = Literal["image", "video", "audio", "document"]
"""Kind of attachment for :meth:`WotpClient.send_media` — wotp supports the
same four kinds on both its whatsmeow and Cloud API backends."""


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

    This is an instance-wide liveness check — see :meth:`WotpClient.get_chats`
    or the dashboard for the connected number's own status (an instance is
    mono-tenant: exactly one WhatsApp number).
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
    """A WhatsApp contact visible to the connected number."""

    jid: str
    name: str | None = None

    model_config = {"populate_by_name": True}


# ─── Conversations & takeover ──────────────────────────────────────

ConversationState = Literal["bot", "human"]
"""State of a :class:`Conversation` — ``"bot"`` by default, ``"human"``
after a takeover."""


class Conversation(BaseModel):
    """A contact's WhatsApp conversation thread — one per phone number,
    created automatically on first inbound contact."""

    id: str
    phone: str
    state: ConversationState
    created_at: datetime = Field(alias="created_at")
    updated_at: datetime = Field(alias="updated_at")

    model_config = {"populate_by_name": True}


class ConversationMessage(BaseModel):
    """One entry in :meth:`WotpClient.get_conversation_messages`'s merged,
    chronological thread — inbound replies, outbound sends, and OTP sends
    all show up here. ``kind`` is ``"otp"``/``"text"``/``"media"`` for
    outbound entries, or an inbound media message's kind
    (``"image"``/``"video"``/``"audio"``/``"document"``); ``None`` for a
    plain inbound text/location message.
    """

    direction: Literal["inbound", "outbound"]
    kind: str | None = None
    content: str
    push_name: str | None = Field(default=None, alias="push_name")
    media_mime_type: str | None = Field(default=None, alias="media_mime_type")
    message_id: str | None = Field(default=None, alias="message_id")
    status: str | None = None
    at: datetime

    model_config = {"populate_by_name": True}


# ─── Inbound media ──────────────────────────────────────────────────


@dataclass
class MediaFile:
    """Raw bytes of a downloaded inbound media message — an image, video,
    voice note, or document a contact sent in, ready to feed to OCR,
    Whisper, or wherever else your bot needs it."""

    data: bytes
    content_type: str
