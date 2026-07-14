import { apiClient } from "@/lib/api-client";
import { auth } from "@/lib/auth";
import { headers } from "next/headers";
import { NextResponse } from "next/server";

export async function GET(request: Request) {
    try {
        const session = await auth.api.getSession({
            headers: await headers()
        });

        if (!session?.session.activeOrganizationId) {
            return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
        }

        const org = await apiClient.getOrganization(session.session.activeOrganizationId);
        return NextResponse.json(org);
    } catch (error: any) {
        return NextResponse.json({ error: error.message }, { status: 500 });
    }
}

export async function PATCH(request: Request) {
    try {
        const session = await auth.api.getSession({
            headers: await headers()
        });

        if (!session?.session.activeOrganizationId) {
            return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
        }

        const body = await request.json();
        const org = await apiClient.updateOrganization(session.session.activeOrganizationId, body);
        return NextResponse.json(org);
    } catch (error: any) {
        return NextResponse.json({ error: error.message }, { status: 500 });
    }
}
