import { AppLayout } from "@/components/AppLayout";
import { auth } from "@/lib/auth";
import { headers } from "next/headers";
import { redirect } from "next/navigation";

export default async function OrgLayout({
  children,
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = await params;
  // Vérifier la session côté serveur
  const headersList = await headers();
  const session = await auth.api.getSession({
    headers: headersList
  });

  if (!session?.session) {
    redirect('/auth/sign-in');
  }

  // Vérifier que l'utilisateur a accès à cette organisation
  // Better-Auth avec le plugin organization devrait exposer les orgs dans session.user
  // On va d'abord vérifier si l'org existe et si l'user y a accès
  
  // Pour l'instant, on laisse passer et on gérera la validation dans le middleware
  // Le layout fournit juste le contexte org à tous les enfants

  return (
    <AppLayout>
      {children}
    </AppLayout>
  );
}
