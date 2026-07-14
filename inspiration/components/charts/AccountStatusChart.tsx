"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ChartContainer, type ChartConfig } from "@/components/ui/chart";
import { Label, PolarRadiusAxis, RadialBar, RadialBarChart } from "recharts";

interface AccountStatusChartProps {
  connected: number;
  total: number;
}

export function AccountStatusChart({ connected, total }: AccountStatusChartProps) {
  const disconnected = total - connected;
  const percentage = total > 0 ? Math.round((connected / total) * 100) : 0;

  const chartData = [{ connected, disconnected }];
  
  const chartConfig = {
    connected: {
      label: "Connectés",
      color: "hsl(142, 76%, 36%)", // Vert WhatsApp
    },
    disconnected: {
      label: "Déconnectés",
      color: "hsl(0, 0%, 45%)",
    },
  } satisfies ChartConfig;

  return (
    <Card className="bg-card border-border">
      <CardHeader className="pb-2">
        <CardTitle className="text-card-foreground text-base">État des comptes</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col items-center pb-4">
        <ChartContainer 
          config={chartConfig} 
          className="mx-auto aspect-square w-full max-w-[180px]"
        >
          <RadialBarChart 
            data={chartData} 
            endAngle={180} 
            innerRadius={60} 
            outerRadius={100}
          >
            <PolarRadiusAxis tick={false} tickLine={false} axisLine={false}>
              <Label
                content={({ viewBox }) => {
                  if (viewBox && "cx" in viewBox && "cy" in viewBox) {
                    return (
                      <text x={viewBox.cx} y={viewBox.cy} textAnchor="middle">
                        <tspan 
                          x={viewBox.cx} 
                          y={(viewBox.cy || 0) - 10} 
                          className="fill-foreground text-3xl font-bold"
                        >
                          {percentage}%
                        </tspan>
                        <tspan 
                          x={viewBox.cx} 
                          y={(viewBox.cy || 0) + 15} 
                          className="fill-muted-foreground text-sm"
                        >
                          actifs
                        </tspan>
                      </text>
                    );
                  }
                }}
              />
            </PolarRadiusAxis>
            <RadialBar 
              dataKey="connected" 
              stackId="a" 
              cornerRadius={5} 
              fill="var(--color-connected)"
              className="stroke-transparent stroke-2"
            />
            <RadialBar 
              dataKey="disconnected" 
              stackId="a" 
              cornerRadius={5} 
              fill="var(--color-disconnected)"
              className="stroke-transparent stroke-2"
            />
          </RadialBarChart>
        </ChartContainer>
        <div className="mt-2 text-center text-sm text-muted-foreground">
          <span className="text-[#25D366] font-semibold">{connected}</span> sur {total} comptes connectés
        </div>
      </CardContent>
    </Card>
  );
}
