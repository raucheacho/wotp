import { apiClient } from "@/lib/api-client";
import { NextRequest, NextResponse } from "next/server";

/**
 * GET /api/webhooks/deliveries - Get webhook deliveries with optional filters
 */
export async function GET(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url);

    const options: {
      organizationId?: string;
      event?: string;
      success?: boolean;
      limit?: number;
      offset?: number;
      fromDate?: Date;
      toDate?: Date;
    } = {};

    const organizationId = searchParams.get("organizationId");
    if (organizationId) {
      options.organizationId = organizationId;
    }
    const event = searchParams.get("event");
    if (event) {
      options.event = event;
    }
    const success = searchParams.get("success");
    if (success) {
      options.success = success === "true";
    }
    const limit = searchParams.get("limit");
    if (limit) {
      options.limit = parseInt(limit);
    }
    const offset = searchParams.get("offset");
    if (offset) {
      options.offset = parseInt(offset);
    }
    const fromDate = searchParams.get("fromDate");
    if (fromDate) {
      options.fromDate = new Date(fromDate);
    }
    const toDate = searchParams.get("toDate");
    if (toDate) {
      options.toDate = new Date(toDate);
    }

    const deliveries = await apiClient.getWebhookDeliveries(options);
    return NextResponse.json(deliveries);
  } catch (error) {
    console.error("Failed to fetch webhook deliveries:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to fetch deliveries",
      },
      { status: 500 },
    );
  }
}
