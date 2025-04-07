"use client";

import { useState } from "react";
import { Check, Clock, X } from "lucide-react";

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";

export function FunctionInvocationHistory() {
  const [invocations] = useState([
    {
      id: "inv-1",
      timestamp: "2023-04-07 14:32:45",
      duration: "245ms",
      status: "Success",
      requestId: "req-abc123",
    },
    {
      id: "inv-2",
      timestamp: "2023-04-07 13:15:22",
      duration: "189ms",
      status: "Success",
      requestId: "req-def456",
    },
    {
      id: "inv-3",
      timestamp: "2023-04-07 12:48:10",
      duration: "312ms",
      status: "Success",
      requestId: "req-ghi789",
    },
    {
      id: "inv-4",
      timestamp: "2023-04-07 11:22:05",
      duration: "156ms",
      status: "Error",
      requestId: "req-jkl012",
    },
    {
      id: "inv-5",
      timestamp: "2023-04-07 10:05:33",
      duration: "278ms",
      status: "Success",
      requestId: "req-mno345",
    },
  ]);

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Timestamp</TableHead>
          <TableHead>Duration</TableHead>
          <TableHead>Status</TableHead>
          <TableHead>Request ID</TableHead>
          <TableHead className="w-[100px]">Actions</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {invocations.map((invocation) => (
          <TableRow key={invocation.id}>
            <TableCell>{invocation.timestamp}</TableCell>
            <TableCell>
              <div className="flex items-center gap-2">
                <Clock className="h-3 w-3 text-muted-foreground" />
                {invocation.duration}
              </div>
            </TableCell>
            <TableCell>
              <Badge
                variant={
                  invocation.status === "Success" ? "default" : "destructive"
                }
                className={
                  invocation.status === "Success" ? "bg-green-500" : ""
                }
              >
                {invocation.status === "Success" ? (
                  <Check className="mr-1 h-3 w-3" />
                ) : (
                  <X className="mr-1 h-3 w-3" />
                )}
                {invocation.status}
              </Badge>
            </TableCell>
            <TableCell className="font-mono text-xs">
              {invocation.requestId}
            </TableCell>
            <TableCell>
              <div className="flex items-center gap-2">
                <button className="text-xs text-blue-500 hover:underline">
                  View Logs
                </button>
              </div>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
