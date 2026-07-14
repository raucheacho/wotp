import { ADMIN_API_KEY, SERVER_URL } from "@/lib/config";
import { NextRequest, NextResponse } from "next/server";

/**
 * GET /api/organizations/[id]/accounts/summary - Get account summary by status
 */
export async function GET(
  _request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  try {
    const { id } = await params;
    const response = await fetch(
      `${SERVER_URL}/v1/organizations/${id}/accounts/summary`,
      {
        headers: {
          "X-API-Key": ADMIN_API_KEY,
        },
      },
    );

    if (!response.ok) {
      const error = await response
        .json()
        .catch(() => ({ error: "Failed to fetch summary" }));
      return NextResponse.json(error, { status: response.status });
    }

    const data = await response.json();
    return NextResponse.json(data);
  } catch (error) {
    console.error("Failed to fetch account summary:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to fetch summary",
      },
      { status: 500 },
    );
  }
}
