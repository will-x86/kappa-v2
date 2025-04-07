"use client";

import { useTheme } from "next-themes";
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

const data = [
  {
    name: "Mon",
    invocations: 145,
    errors: 5,
  },
  {
    name: "Tue",
    invocations: 232,
    errors: 3,
  },
  {
    name: "Wed",
    invocations: 186,
    errors: 2,
  },
  {
    name: "Thu",
    invocations: 256,
    errors: 8,
  },
  {
    name: "Fri",
    invocations: 312,
    errors: 4,
  },
  {
    name: "Sat",
    invocations: 198,
    errors: 1,
  },
  {
    name: "Sun",
    invocations: 153,
    errors: 0,
  },
];

export function FunctionMetricsChart() {
  const { theme } = useTheme();
  const isDark = theme === "dark";

  return (
    <ResponsiveContainer width="100%" height={350}>
      <BarChart data={data}>
        <CartesianGrid
          strokeDasharray="3 3"
          vertical={false}
          stroke={isDark ? "hsl(240 3.7% 15.9%)" : "hsl(220 13% 91%)"}
        />
        <XAxis
          dataKey="name"
          stroke={isDark ? "hsl(240 5% 64.9%)" : "hsl(240 3.8% 46.1%)"}
          fontSize={12}
          tickLine={false}
          axisLine={false}
        />
        <YAxis
          stroke={isDark ? "hsl(240 5% 64.9%)" : "hsl(240 3.8% 46.1%)"}
          fontSize={12}
          tickLine={false}
          axisLine={false}
          tickFormatter={(value) => `${value}`}
        />
        <Tooltip
          contentStyle={{
            backgroundColor: isDark ? "hsl(240 10% 3.9%)" : "white",
            borderColor: isDark ? "hsl(240 3.7% 15.9%)" : "hsl(220 13% 91%)",
            borderRadius: "6px",
            fontSize: "12px",
          }}
        />
        <Bar
          dataKey="invocations"
          fill="hsl(217.2 91.2% 59.8%)"
          radius={[4, 4, 0, 0]}
          name="Invocations"
        />
        <Bar
          dataKey="errors"
          fill="hsl(0 84.2% 60.2%)"
          radius={[4, 4, 0, 0]}
          name="Errors"
        />
      </BarChart>
    </ResponsiveContainer>
  );
}
