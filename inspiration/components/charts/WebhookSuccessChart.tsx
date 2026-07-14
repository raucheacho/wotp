"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ChartContainer, type ChartConfig } from "@/components/ui/chart";
import { Label, PolarRadiusAxis, RadialBar, RadialBarChart } from "recharts";

interface WebhookSuccessChartProps {
  successful: number;
  failed: number;
}

export function WebhookSuccessChart({ successful, failed }: WebhookSuccessChartProps) {
  const total = successful + failed;
  const successRate = total > 0 ? Math.round((successful / total) * 100) : 0;

  const chartData = [{ successful, failed }];
  
  const chartConfig = {
    successful: {
      label: "Succès",
      color: "hsl(142, 76%, 36%)", // Vert
    },
    failed: {
      label: "Échecs",
      color: "hsl(0, 84%, 60%)", // Rouge
    },
  } satisfies ChartConfig;

  return (
    <Card className="bg-card border-border">
      <CardHeader className="pb-2">
        <CardTitle className="text-card-foreground text-base">Performance Webhooks</CardTitle>
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
                          {successRate}%
                        </tspan>
                        <tspan 
                          x={viewBox.cx} 
                          y={(viewBox.cy || 0) + 15} 
                          className="fill-muted-foreground text-sm"
                        >
                          succès
                        </tspan>
                      </text>
                    );
                  }
                }}
              />
            </PolarRadiusAxis>
            <RadialBar 
              dataKey="successful" 
              stackId="a" 
              cornerRadius={5} 
              fill="var(--color-successful)"
              className="stroke-transparent stroke-2"
            />
            <RadialBar 
              dataKey="failed" 
              stackId="a" 
              cornerRadius={5} 
              fill="var(--color-failed)"
              className="stroke-transparent stroke-2"
            />
          </RadialBarChart>
        </ChartContainer>
        <div className="mt-2 text-center text-sm text-muted-foreground">
          <span className="text-[#25D366] font-semibold">{successful.toLocaleString()}</span> livrés · {" "}
          <span className="text-destructive">{failed.toLocaleString()}</span> échecs
        </div>
      </CardContent>
    </Card>
  );
}
