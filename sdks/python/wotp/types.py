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
    """Response from GET /health."""

    status: str = Field(description="WhatsApp connection status.")
    phone: str = Field(description="Connected phone number.")
    uptime_seconds: int = Field(
        alias="uptime_seconds",
        description="Uptime in seconds.",
    )

    model_config = {"populate_by_name": True}
