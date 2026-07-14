import { auth } from "@/lib/auth";
import { NextResponse, type NextRequest } from "next/server";

export async function proxy(request: NextRequest) {
  const pathname = request.nextUrl.pathname;
  
  // Public routes that don't require authentication or org context
  const publicRoutes = [
    '/auth/sign-in', 
    '/auth/sign-up', 
    '/auth/forgot-password', 
    '/auth/reset-password',
    '/onboarding',
    '/connect', // QR code public page
  ];
  
  const isPublicRoute = publicRoutes.some(route => pathname.startsWith(route));
  
  // Skip all checks for public routes
  if (isPublicRoute) {
    return NextResponse.next();
  }

  // Root path - will be handled by page.tsx (redirect to org)
  if (pathname === '/') {
    return NextResponse.next();
  }
  
  // Check for Better-Auth session
  const session = await auth.api.getSession({
    headers: request.headers
  });
  
  // Redirect to sign-in if no session exists
  if (!session?.session) {
    const signInUrl = new URL('/auth/sign-in', request.url);
    signInUrl.searchParams.set('from', pathname);
    return NextResponse.redirect(signInUrl);
  }

  // Extract orgId from URL path (format: /[orgId]/...)
  const pathSegments = pathname.substring(1).split("/");
  const orgId = pathSegments[0];
  const applicationPath = "/" + pathSegments.slice(1).join("/");

  // Validate orgId format (UUID v4)
  const uuidRegex = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
  
  if (!uuidRegex.test(orgId)) {
    // Not a valid UUID, might be another route (like /api, /admin system pages, etc.)
    // Let it pass through, or return 404 if it's meant to be an org route
    return NextResponse.next();
  }

  // Verify user has access to this organization
  // Better-Auth organization plugin should expose user's organizations
  // We need to check if the user is a member of this org
  
  // Note: Better-Auth session might not directly expose organizations
  // We'll need to fetch from DB or use a custom query
  // For now, we'll let the page components handle the detailed check
  // and just validate that it's a proper org format
  
  // TODO: Add actual organization membership check here
  // This would require querying the member table from Better-Auth
  // or adding custom session data
  
  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!api|_next/static|_next/image|favicon.ico).*)"],
};
