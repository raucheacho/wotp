import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Account } from "@/lib/api-client";
import { Zap } from "lucide-react";

export function VisualQuota({ account }: { account: Account }) {
  const limit = account.rateLimit || 30;
  // Assumption: Normal user sends < 5 msgs/min, Power user ~20.
  // This helps visualize where the limit stands.
  
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm font-medium text-foreground flex items-center gap-2">
          <Zap className="w-4 h-4 text-purple-500" /> Quota & Limites
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          <div>
            <div className="flex justify-between text-sm mb-2">
              <span className="text-muted-foreground">Rate Limit</span>
              <span className="text-foreground font-medium">{limit} msg/min</span>
            </div>
            {/* Visual Bar representation */}
            <div className="h-2 bg-muted rounded-full overflow-hidden">
               {/* Just a visual indicator of capacity - we don't have realtime usage yet */}
               <div className="h-full bg-purple-500/20 w-full relative">
                  <div className="absolute top-0 bottom-0 right-0 w-1 bg-purple-500"></div>
               </div>
            </div>
            <p className="text-xs text-muted-foreground mt-2">
              Maximum de messages autorisés par minute pour ce compte.
            </p>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
