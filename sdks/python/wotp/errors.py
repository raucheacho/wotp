"""Custom exception hierarchy for the Wotp SDK.

Business errors are raised as typed exceptions so callers can catch
specific failure modes without parsing HTTP status codes.
"""


class WotpError(Exception):
    """Base exception for all Wotp SDK errors."""

    def __init__(self, message: str = "Wotp SDK error") -> None:
        self.message = message
        super().__init__(self.message)


class RateLimitError(WotpError):
    """Raised when the API returns HTTP 429.

    The phone number or IP has exceeded the configured rate limit.
    """

    def __init__(
        self,
        message: str = "Rate limit exceeded",
        retry_after: int | None = None,
    ) -> None:
        self.retry_after = retry_after
        super().__init__(message)


class ExpiredTokenError(WotpError):
    """Raised when verification is attempted with an expired token."""

    def __init__(self, message: str = "OTP token has expired") -> None:
        super().__init__(message)


class InvalidCodeError(WotpError):
    """Raised when the OTP code is incorrect."""

    def __init__(
        self,
        message: str = "Invalid OTP code",
        attempts_remaining: int | None = None,
    ) -> None:
        self.attempts_remaining = attempts_remaining
        super().__init__(message)
