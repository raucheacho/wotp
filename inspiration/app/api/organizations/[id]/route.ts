import { apiClient } from "@/lib/api-client";
import { NextRequest, NextResponse } from "next/server";

/**
 * GET /api/organizations/[id] - Get organization by ID
 */
export async function GET(
  _request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  try {
    const { id } = await params;
    const organization = await apiClient.getOrganization(id);
    return NextResponse.json(organization);
  } catch (error) {
    console.error("Failed to fetch organization:", error);
    const status =
      error instanceof Error && error.message.includes("not found") ? 404 : 500;
    return NextResponse.json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to fetch organization",
      },
      { status },
    );
  }
}

/**
 * PATCH /api/organizations/[id] - Update organization
 */
export async function PATCH(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  try {
    const { id } = await params;
    const body = await request.json();
    const updated = await apiClient.updateOrganization(id, body);
    return NextResponse.json(updated);
  } catch (error) {
    console.error("Failed to update organization:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to update organization",
      },
      { status: 500 },
    );
  }
}

/**
 * DELETE /api/organizations/[id] - Delete organization
 */
export async function DELETE(
  _request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  try {
    const { id } = await params;
    await apiClient.deleteOrganization(id);
    return NextResponse.json({ success: true });
  } catch (error) {
    console.error("Failed to delete organization:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to delete organization",
      },
      { status: 500 },
    );
  }
}
