import { apiClient } from "@/lib/api-client";
import { NextRequest, NextResponse } from "next/server";

/**
 * GET /api/accounts/[id] - Get account by ID
 */
export async function GET(
  _request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  try {
    const { id } = await params;
    const account = await apiClient.getAccount(id);
    return NextResponse.json(account);
  } catch (error) {
    console.error("Failed to fetch account:", error);
    const status =
      error instanceof Error && error.message.includes("not found") ? 404 : 500;
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to fetch account",
      },
      { status },
    );
  }
}

/**
 * PATCH /api/accounts/[id] - Update account
 */
export async function PATCH(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  try {
    const { id } = await params;
    const body = await request.json();
    const updated = await apiClient.updateAccount(id, body);
    return NextResponse.json(updated);
  } catch (error) {
    console.error("Failed to update account:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to update account",
      },
      { status: 500 },
    );
  }
}

/**
 * DELETE /api/accounts/[id] - Delete account
 */
export async function DELETE(
  _request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  try {
    const { id } = await params;
    await apiClient.deleteAccount(id);
    return NextResponse.json({ success: true });
  } catch (error) {
    console.error("Failed to delete account:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to delete account",
      },
      { status: 500 },
    );
  }
}
