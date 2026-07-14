import { auth } from "@/lib/auth";
import { member } from "@/lib/auth-schema";
import { db } from "@/lib/db";
import { eq } from "drizzle-orm";
import { headers } from "next/headers";
import { redirect } from "next/navigation";

export default async function RootPage() {
  const headersList = await headers();
  const session = await auth.api.getSession({
    headers: headersList
  });

  // If not authenticated, redirect to sign-in
  if (!session?.session) {
    redirect('/auth/sign-in');
  }

  const userId = session.session.userId;

  // Fetch user's organizations from member table
  const userMemberships = await db
    .select()
    .from(member)
    .where(eq(member.userId, userId))
    .limit(1);

  // If no organizations, redirect to onboarding
  if (userMemberships.length === 0) {
    redirect('/onboarding/create-organization');
  }

  // Redirect to first organization's dashboard
  const firstOrgId = userMemberships[0].organizationId;
  redirect(`/${firstOrgId}`);
}
