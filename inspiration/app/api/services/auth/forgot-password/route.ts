import { auth } from "@/lib/auth";
import { passwordResetLimiter } from "@/lib/rate-limit";
import { type NextRequest, NextResponse } from "next/server";

function validateEmail(email: string): boolean {
  const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
  return emailRegex.test(email);
}

function validateRedirectUrl(url: string | undefined): string {
  const defaultRedirect = "/auth/reset-password";
  
  if (!url) return defaultRedirect;
  
  // Only allow relative URLs starting with /
  if (!url.startsWith('/')) return defaultRedirect;
  
  // Block protocol-relative URLs (//evil.com)
  if (url.startsWith('//')) return defaultRedirect;
  
  return url;
}

export async function POST(req: NextRequest) {
  try {
    // Rate limiting with Upstash Redis
    const ip = (req as any).ip ?? req.headers.get('x-forwarded-for') ?? req.headers.get('x-real-ip') ?? '127.0.0.1';
    const { success, limit, remaining, reset } = await passwordResetLimiter.limit(ip);
    
    if (!success) {
      const retryAfter = Math.ceil((reset - Date.now()) / 1000);
      return NextResponse.json(
        { error: "Too many requests. Please try again later." },
        { 
          status: 429,
          headers: {
            'Retry-After': retryAfter.toString(),
            'X-RateLimit-Limit': limit.toString(),
            'X-RateLimit-Remaining': remaining.toString(),
            'X-RateLimit-Reset': new Date(reset).toISOString(),
          }
        }
      );
    }

    const { email, redirectTo } = await req.json();

    // Validate email format
    if (!email || !validateEmail(email)) {
      return NextResponse.json(
        { error: "Invalid email format" },
        { status: 400 }
      );
    }
    
    // Validate and sanitize redirectTo
    const safeRedirectTo = validateRedirectUrl(redirectTo);

    // Use the correct Better-Auth API method: requestPasswordReset
    const result = await auth.api.requestPasswordReset({
      body: {
        email,
        redirectTo: safeRedirectTo
      }
    });

    // Always return success to prevent email enumeration
    // Better-Auth handles the case where email doesn't exist internally
    return NextResponse.json({ 
      success: true,
      message: "If the email exists, a reset link has been sent."
    }, {
      headers: {
        'X-RateLimit-Limit': limit.toString(),
        'X-RateLimit-Remaining': remaining.toString(),
        'X-RateLimit-Reset': new Date(reset).toISOString(),
      }
    });
  } catch (error: any) {
    console.error("Password Reset Error:", error);
    
    return NextResponse.json(
      { error: error?.message || "Failed to process request" },
      { status: 500 }
    );
  }
}
