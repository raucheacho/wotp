"use client";
import { apiClient } from "@/lib/api-client";
import type { Organization } from "@/types";
import dynamic from "next/dynamic";
import { useParams } from "next/navigation";
import { useEffect, useState } from "react";
import "swagger-ui-react/swagger-ui.css";

// Dynamic import for SwaggerUI to avoid SSR issues
const SwaggerUI: any = dynamic(() => import("swagger-ui-react"), { ssr: false });

export default function ApiDocsPage() {
  const params = useParams();
  const organizationId = params.orgId as string;
  const [organization, setOrganization] = useState<Organization | null>(null);
  
  const apiUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8000";
  const specUrl = `${apiUrl}/docs/openapi.json`;

  useEffect(() => {
    if (organizationId) {
      apiClient.getOrganization(organizationId).then(setOrganization);
    }
  }, [organizationId]);

  return (
    <div className="space-y-6 h-full flex flex-col">
      <div className="flex flex-col gap-2">
        <h1 className="text-3xl font-bold tracking-tight text-white">
          API Documentation
        </h1>
        <p className="text-zinc-400">
          Interactive API reference for {organization?.name || "your organization"}.
          {organization?.apiKey ? (
            <span className="ml-2 text-emerald-400 text-sm">
              Authenticated ✓
            </span>
          ) : (
            <span className="ml-2 text-yellow-500 text-sm">
              No API Key found (Read-Only)
            </span>
          )}
        </p>
      </div>

      <div className="flex-1 bg-white rounded-lg shadow-xl overflow-hidden swagger-dark-mode-fixes">
        <SwaggerUI 
          url={specUrl}
          requestInterceptor={(req: any) => {
            if (organization?.apiKey) {
              req.headers["X-API-Key"] = organization.apiKey;
            }
            return req;
          }}
          defaultModelsExpandDepth={-1} // Hide models by default
          displayOperationId={true}
        />
      </div>
      
      {/* Custom styles to force Swagger UI into a dark(ish) theme integration if needed, 
          or just ensure it fits in the layout container */}
      <style jsx global>{`
        .swagger-ui .info { margin: 20px 0; }
        .swagger-ui .scheme-container { background: transparent; box-shadow: none; }
      `}</style>
    </div>
  );
}
