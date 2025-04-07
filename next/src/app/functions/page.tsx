import Link from "next/link";
import { Code, MoreHorizontal, Plus, Search } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";

export default function FunctionsPage() {
  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div>
          <h2 className="text-3xl font-bold tracking-tight">Functions</h2>
          <p className="text-muted-foreground">
            Manage your serverless functions
          </p>
        </div>
        <Button asChild>
          <Link href="/functions/new">
            <Plus className="mr-2 h-4 w-4" />
            Create Function
          </Link>
        </Button>
      </div>

      <Card>
        <CardHeader className="flex flex-row items-center">
          <div className="grid gap-2">
            <CardTitle>All Functions</CardTitle>
            <CardDescription>
              View and manage all your serverless functions
            </CardDescription>
          </div>
          <div className="ml-auto flex items-center gap-2">
            <div className="relative">
              <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                type="search"
                placeholder="Search functions..."
                className="w-[200px] pl-8 md:w-[260px]"
              />
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Runtime</TableHead>
                <TableHead>Memory</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Last Updated</TableHead>
                <TableHead className="w-[50px]"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {functions.map((func) => (
                <TableRow key={func.id}>
                  <TableCell className="font-medium">
                    <div className="flex items-center gap-2">
                      <Code className="h-4 w-4 text-muted-foreground" />
                      <Link
                        href={`/functions/${func.id}`}
                        className="hover:underline"
                      >
                        {func.name}
                      </Link>
                    </div>
                  </TableCell>
                  <TableCell>{func.runtime}</TableCell>
                  <TableCell>{func.memory}MB</TableCell>
                  <TableCell>
                    <Badge
                      variant={
                        func.status === "Active" ? "default" : "secondary"
                      }
                      className={func.status === "Active" ? "bg-green-500" : ""}
                    >
                      {func.status}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {func.updatedAt}
                  </TableCell>
                  <TableCell>
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon">
                          <MoreHorizontal className="h-4 w-4" />
                          <span className="sr-only">Open menu</span>
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuLabel>Actions</DropdownMenuLabel>
                        <DropdownMenuItem asChild>
                          <Link href={`/functions/${func.id}`}>
                            View Details
                          </Link>
                        </DropdownMenuItem>
                        <DropdownMenuItem asChild>
                          <Link href={`/functions/${func.id}/edit`}>
                            Edit Function
                          </Link>
                        </DropdownMenuItem>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem className="text-red-500">
                          Delete Function
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}

const functions = [
  {
    id: "1",
    name: "image-processor",
    runtime: "Node.js 20.x",
    memory: 512,
    status: "Active",
    updatedAt: "2 hours ago",
  },
  {
    id: "2",
    name: "data-transformer",
    runtime: "Python 3.11",
    memory: 256,
    status: "Active",
    updatedAt: "5 hours ago",
  },
  {
    id: "3",
    name: "notification-sender",
    runtime: "Node.js 20.x",
    memory: 128,
    status: "Active",
    updatedAt: "1 day ago",
  },
  {
    id: "4",
    name: "payment-webhook",
    runtime: "Node.js 18.x",
    memory: 256,
    status: "Inactive",
    updatedAt: "2 days ago",
  },
  {
    id: "5",
    name: "user-authenticator",
    runtime: "Node.js 20.x",
    memory: 256,
    status: "Active",
    updatedAt: "3 days ago",
  },
];
