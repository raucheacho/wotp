import { apiClient } from "@/lib/api-client";
import { NextRequest, NextResponse } from "next/server";

/**
 * GET /api/audit/logs - Get audit logs with optional filters
 */
export async function GET(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url);

    const options: {
      organizationId?: string;
      action?: string;
      resourceType?: string;
      resourceId?: string;
      limit?: number;
      offset?: number;
      fromDate?: Date;
      toDate?: Date;
    } = {};

    const organizationId = searchParams.get("organizationId");
    if (organizationId) {
      options.organizationId = organizationId;
    }
    const action = searchParams.get("action");
    if (action) {
      options.action = action;
    }
    const resourceType = searchParams.get("resourceType");
    if (resourceType) {
      options.resourceType = resourceType;
    }
    const resourceId = searchParams.get("resourceId");
    if (resourceId) {
      options.resourceId = resourceId;
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

    const logs = await apiClient.getAuditLogs(options);
    return NextResponse.json(logs);
  } catch (error) {
    console.error("Failed to fetch audit logs:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to fetch audit logs",
      },
      { status: 500 },
    );
  }
}
