import { auth } from "@/lib/auth";
import { ADMIN_API_KEY, SERVER_URL } from "@/lib/config";
import { db } from "@/lib/db";
import { accounts } from "@/lib/schema";
import { eq } from "drizzle-orm";
import { headers } from "next/headers";
import { NextRequest, NextResponse } from "next/server";

async function handler(req: NextRequest, { params }: { params: Promise<{ all: string[] }> }) {
    const session = await auth.api.getSession({
        headers: await headers()
    });
    if (!session) {
        return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
    }

    const { all } = await params;
    const path = all.join("/");
    const url = `${SERVER_URL}/${path}${req.nextUrl.search}`;
    const method = req.method;

    const activeOrgId = session.session.activeOrganizationId;
    const isAdmin = session.user.role === "admin";

    // 1. Security: Ensure Active Organization
    if (!activeOrgId && !isAdmin) {
         return NextResponse.json({ error: "No active organization" }, { status: 403 });
    }

    // 2. Security: IDOR Check for /accounts/:id
    if (all[0] === "accounts" && all[1] && !isAdmin) {
        // If getting specific account
        // Check if it's a UUID (to avoid matching 'summary' etc if any)
        if (all[1].match(/^[0-9a-fA-F-]{36}$/)) {
             const accountId = all[1];
             const account = await db.query.accounts.findFirst({
                 where: eq(accounts.id, accountId),
                 columns: { organizationId: true }
             });
             if (!account || account.organizationId !== activeOrgId) {
                  return NextResponse.json({ error: "Access Denied" }, { status: 403 });
             }
        }
    }
    
    // 3. Security: Enforce Organization Isolation on Lists
    if (all[0] === "accounts" && !all[1] && method === "GET" && !isAdmin) {
        const urlObj = new URL(req.url); // Use req.url to get search params from incoming request
        const requestedOrg = urlObj.searchParams.get("organizationId");
        
        // If user tries to filter by another org, block it.
        // Or if they don't provide it, we could inject it?
        // Current API client sends it.
        if (requestedOrg && requestedOrg !== activeOrgId) {
              return NextResponse.json({ error: "Organization Mismatch" }, { status: 403 });
        }
        // If not provided, the server will return ALL accounts (if admin) or fail?
        // Baileys server accounts endpoint filters by organizationId if provided.
        // If NOT provided, it might return all?
        // We should FORCE it.
        // But we can't easily modify URL here without re-constructing.
        // We'll trust the Client sends it (as validated by `Organization Mismatch`),
        // OR we reject if missing?
        if (!requestedOrg) {
             return NextResponse.json({ error: "organizationId is required" }, { status: 400 });
        }
    }

    const body = method !== "GET" && method !== "HEAD" ? await req.blob() : undefined;
    
    try {
        const res = await fetch(url, {
            method,
            headers: {
                "Content-Type": req.headers.get("Content-Type") || "application/json",
                "Authorization": `Bearer ${ADMIN_API_KEY}`,
                "X-Organization-Id": activeOrgId || "",
            },
            body,
        });

        // Copy headers
        const accessControlHeaders = new Headers(res.headers);
        // Ensure CORS if needed (though proxying usually avoids it)

        return new NextResponse(res.body, {
            status: res.status,
            statusText: res.statusText,
            headers: accessControlHeaders
        });
    } catch (error) {
        console.error("Proxy Error:", error);
        return NextResponse.json({ error: "Upstream Error" }, { status: 502 });
    }
}

export { handler as DELETE, handler as GET, handler as PATCH, handler as POST, handler as PUT };

