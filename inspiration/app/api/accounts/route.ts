import { apiClient } from "@/lib/api-client";
import { NextRequest, NextResponse } from "next/server";

/**
 * GET /api/accounts - List all accounts or filter by organization
 */
export async function GET(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url);
    const organizationId = searchParams.get("organizationId");

    const accounts = await apiClient.listAccounts(organizationId || undefined);
    return NextResponse.json(accounts);
  } catch (error) {
    console.error("Failed to fetch accounts:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to fetch accounts",
      },
      { status: 500 },
    );
  }
}

/**
 * POST /api/accounts - Create a new account
 */
export async function POST(request: NextRequest) {
  try {
    const body = await request.json();
    const account = await apiClient.createAccount(body);
    return NextResponse.json(account, { status: 201 });
  } catch (error) {
    console.error("Failed to create account:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to create account",
      },
      { status: 500 },
    );
  }
}
