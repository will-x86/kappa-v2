import Link from "next/link";
import {
  ArrowLeft,
  Clock,
  Code,
  Copy,
  Edit,
  ExternalLink,
  Play,
  Trash,
} from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { FunctionCodeEditor } from "@/components/function-code-editor";
import { FunctionInvocationHistory } from "@/components/function-invocation-history";

export default async function FunctionDetailPage({
  params,
}: {
  params: { id: string };
}) {
  const { id } = await params;
  const functionData = {
    id: id,
    name: "image-processor",
    description:
      "Processes and optimizes images uploaded to the storage bucket",
    runtime: "Node.js 20.x",
    memory: 512,
    timeout: 30,
    status: "Active",
    createdAt: "2023-12-15",
    updatedAt: "2 hours ago",
    url: "https://api.example.com/functions/image-processor",
    environment: [
      { key: "STORAGE_BUCKET", value: "my-app-uploads" },
      { key: "IMAGE_QUALITY", value: "85" },
    ],
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="outline" size="icon" asChild>
          <Link href="/functions">
            <ArrowLeft className="h-4 w-4" />
            <span className="sr-only">Back</span>
          </Link>
        </Button>
        <div>
          <h2 className="text-3xl font-bold tracking-tight">
            {functionData.name}
          </h2>
          <p className="text-muted-foreground">{functionData.description}</p>
        </div>
        <div className="ml-auto flex items-center gap-2">
          <Button variant="outline" size="sm">
            <Edit className="mr-2 h-4 w-4" />
            Edit
          </Button>
          <Button variant="outline" size="sm">
            <Play className="mr-2 h-4 w-4" />
            Invoke
          </Button>
          <Button variant="destructive" size="sm">
            <Trash className="mr-2 h-4 w-4" />
            Delete
          </Button>
        </div>
      </div>

      <div className="grid gap-6 md:grid-cols-7">
        <div className="md:col-span-5">
          <Tabs defaultValue="code">
            <TabsList>
              <TabsTrigger value="code">Code</TabsTrigger>
              <TabsTrigger value="invocations">Invocation History</TabsTrigger>
              <TabsTrigger value="logs">Logs</TabsTrigger>
              <TabsTrigger value="configuration">Configuration</TabsTrigger>
            </TabsList>
            <TabsContent value="code" className="mt-4">
              <Card>
                <CardHeader>
                  <CardTitle>Function Code</CardTitle>
                  <CardDescription>
                    Edit your function code directly in the browser
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <FunctionCodeEditor />
                  <div className="mt-4 flex justify-end">
                    <Button>Save Changes</Button>
                  </div>
                </CardContent>
              </Card>
            </TabsContent>
            <TabsContent value="invocations" className="mt-4">
              <Card>
                <CardHeader>
                  <CardTitle>Invocation History</CardTitle>
                  <CardDescription>
                    Recent invocations of your function
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <FunctionInvocationHistory />
                </CardContent>
              </Card>
            </TabsContent>
            <TabsContent value="logs" className="mt-4">
              <Card>
                <CardHeader>
                  <CardTitle>Function Logs</CardTitle>
                  <CardDescription>
                    View logs from your function executions
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <div className="rounded-md bg-black p-4 font-mono text-sm text-green-400">
                    <p>2023-04-07 14:32:45 INFO Starting function execution</p>
                    <p>
                      2023-04-07 14:32:45 INFO Processing image: example.jpg
                    </p>
                    <p>2023-04-07 14:32:46 INFO Image resized to 800x600</p>
                    <p>2023-04-07 14:32:46 INFO Optimized image size: 245KB</p>
                    <p>
                      2023-04-07 14:32:47 INFO Uploaded to storage:
                      optimized/example.jpg
                    </p>
                    <p>
                      2023-04-07 14:32:47 INFO Function execution completed
                      successfully
                    </p>
                  </div>
                </CardContent>
              </Card>
            </TabsContent>
            <TabsContent value="configuration" className="mt-4">
              <Card>
                <CardHeader>
                  <CardTitle>Function Configuration</CardTitle>
                  <CardDescription>
                    Configure your function settings
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-6">
                  <div className="grid gap-4 md:grid-cols-2">
                    <div>
                      <h3 className="text-sm font-medium">Runtime</h3>
                      <p className="text-sm text-muted-foreground">
                        {functionData.runtime}
                      </p>
                    </div>
                    <div>
                      <h3 className="text-sm font-medium">Memory</h3>
                      <p className="text-sm text-muted-foreground">
                        {functionData.memory}MB
                      </p>
                    </div>
                    <div>
                      <h3 className="text-sm font-medium">Timeout</h3>
                      <p className="text-sm text-muted-foreground">
                        {functionData.timeout} seconds
                      </p>
                    </div>
                    <div>
                      <h3 className="text-sm font-medium">Status</h3>
                      <Badge
                        variant={
                          functionData.status === "Active"
                            ? "default"
                            : "secondary"
                        }
                        className={
                          functionData.status === "Active" ? "bg-green-500" : ""
                        }
                      >
                        {functionData.status}
                      </Badge>
                    </div>
                  </div>

                  <Separator />

                  <div>
                    <h3 className="mb-2 text-sm font-medium">
                      Environment Variables
                    </h3>
                    <div className="rounded-md border">
                      <div className="grid grid-cols-2 border-b p-3">
                        <div className="font-medium">Key</div>
                        <div className="font-medium">Value</div>
                      </div>
                      {functionData.environment.map((env, index) => (
                        <div
                          key={index}
                          className="grid grid-cols-2 items-center p-3"
                        >
                          <div>{env.key}</div>
                          <div className="flex items-center gap-2">
                            <span>{env.value}</span>
                            <Button
                              variant="ghost"
                              size="icon"
                              className="h-6 w-6"
                            >
                              <Copy className="h-3 w-3" />
                              <span className="sr-only">Copy value</span>
                            </Button>
                          </div>
                        </div>
                      ))}
                    </div>
                    <Button variant="outline" size="sm" className="mt-2">
                      Add Environment Variable
                    </Button>
                  </div>

                  <Separator />

                  <div>
                    <h3 className="mb-2 text-sm font-medium">Function URL</h3>
                    <div className="flex items-center gap-2 rounded-md border p-3">
                      <code className="text-sm">{functionData.url}</code>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="ml-auto h-6 w-6"
                      >
                        <Copy className="h-3 w-3" />
                        <span className="sr-only">Copy URL</span>
                      </Button>
                      <Button variant="ghost" size="icon" className="h-6 w-6">
                        <ExternalLink className="h-3 w-3" />
                        <span className="sr-only">Open URL</span>
                      </Button>
                    </div>
                  </div>
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>
        </div>

        <div className="md:col-span-2">
          <div className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>Function Details</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div>
                  <h3 className="text-sm font-medium">Created</h3>
                  <p className="text-sm text-muted-foreground">
                    {functionData.createdAt}
                  </p>
                </div>
                <div>
                  <h3 className="text-sm font-medium">Last Updated</h3>
                  <p className="text-sm text-muted-foreground">
                    {functionData.updatedAt}
                  </p>
                </div>
                <div>
                  <h3 className="text-sm font-medium">Runtime</h3>
                  <div className="flex items-center gap-2">
                    <Code className="h-4 w-4 text-muted-foreground" />
                    <p className="text-sm text-muted-foreground">
                      {functionData.runtime}
                    </p>
                  </div>
                </div>
                <div>
                  <h3 className="text-sm font-medium">Timeout</h3>
                  <div className="flex items-center gap-2">
                    <Clock className="h-4 w-4 text-muted-foreground" />
                    <p className="text-sm text-muted-foreground">
                      {functionData.timeout} seconds
                    </p>
                  </div>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Recent Metrics</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div>
                  <h3 className="text-sm font-medium">Invocations (24h)</h3>
                  <p className="text-2xl font-bold">128</p>
                </div>
                <div>
                  <h3 className="text-sm font-medium">Avg. Duration</h3>
                  <p className="text-2xl font-bold">215ms</p>
                </div>
                <div>
                  <h3 className="text-sm font-medium">Error Rate</h3>
                  <p className="text-2xl font-bold">0.5%</p>
                </div>
                <Button variant="outline" className="w-full" asChild>
                  <Link href="#">View Full Metrics</Link>
                </Button>
              </CardContent>
            </Card>
          </div>
        </div>
      </div>
    </div>
  );
}
