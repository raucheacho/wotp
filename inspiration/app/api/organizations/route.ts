import { apiClient } from "@/lib/api-client";
import { NextRequest, NextResponse } from "next/server";

/**
 * GET /api/organizations - List all organizations
 */
export async function GET() {
  try {
    const organizations = await apiClient.listOrganizations();
    return NextResponse.json(organizations);
  } catch (error) {
    console.error("Failed to fetch organizations:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to fetch organizations",
      },
      { status: 500 },
    );
  }
}

/**
 * POST /api/organizations - Create a new organization
 */
export async function POST(request: NextRequest) {
  try {
    const body = await request.json();
    const organization = await apiClient.createOrganization(body);
    return NextResponse.json(organization, { status: 201 });
  } catch (error) {
    console.error("Failed to create organization:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to create organization",
      },
      { status: 500 },
    );
  }
}
