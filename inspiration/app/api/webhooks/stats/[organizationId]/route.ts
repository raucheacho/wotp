import { apiClient } from "@/lib/api-client";
import { NextRequest, NextResponse } from "next/server";

/**
 * GET /api/webhooks/stats/[organizationId] - Get webhook delivery statistics
 */
export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ organizationId: string }> },
) {
  try {
    const { organizationId } = await params;
    const { searchParams } = new URL(request.url);
    const days = parseInt(searchParams.get("days") || "30");

    const stats = await apiClient.getWebhookDeliveryStats(organizationId, days);
    return NextResponse.json(stats);
  } catch (error) {
    console.error("Failed to fetch webhook stats:", error);
    return NextResponse.json(
      {
        error: error instanceof Error ? error.message : "Failed to fetch stats",
      },
      { status: 500 },
    );
  }
}
