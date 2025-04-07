"use client";

import type React from "react";

import { useState } from "react";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { FunctionCodeEditor } from "@/components/function-code-editor";

export default function CreateFunctionPage() {
  const [formData, setFormData] = useState({
    name: "",
    description: "",
    runtime: "nodejs20.x",
    memory: "256",
    timeout: "30",
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    console.log("Form submitted:", formData);
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
          <h2 className="text-3xl font-bold tracking-tight">Create Function</h2>
          <p className="text-muted-foreground">
            Create a new serverless function
          </p>
        </div>
      </div>

      <form onSubmit={handleSubmit}>
        <Card>
          <CardHeader>
            <CardTitle>Function Details</CardTitle>
            <CardDescription>
              Provide basic information about your function
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            <div className="grid gap-3">
              <Label htmlFor="name">Function Name</Label>
              <Input
                id="name"
                placeholder="my-function"
                value={formData.name}
                onChange={(e) =>
                  setFormData({ ...formData, name: e.target.value })
                }
                required
              />
              <p className="text-xs text-muted-foreground">
                Function names can only contain letters, numbers, hyphens, and
                underscores
              </p>
            </div>

            <div className="grid gap-3">
              <Label htmlFor="description">Description</Label>
              <Textarea
                id="description"
                placeholder="Describe what your function does"
                value={formData.description}
                onChange={(e) =>
                  setFormData({ ...formData, description: e.target.value })
                }
                className="min-h-[100px]"
              />
            </div>

            <div className="grid gap-6 md:grid-cols-3">
              <div className="grid gap-3">
                <Label htmlFor="runtime">Runtime</Label>
                <Select
                  value={formData.runtime}
                  onValueChange={(value) =>
                    setFormData({ ...formData, runtime: value })
                  }
                >
                  <SelectTrigger id="runtime">
                    <SelectValue placeholder="Select runtime" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="nodejs20.x">Node.js 20.x</SelectItem>
                    <SelectItem value="nodejs18.x">Node.js 18.x</SelectItem>
                    <SelectItem value="python3.11">Python 3.11</SelectItem>
                    <SelectItem value="python3.9">Python 3.9</SelectItem>
                    <SelectItem value="java17">Java 17</SelectItem>
                    <SelectItem value="go1.x">Go 1.x</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="grid gap-3">
                <Label htmlFor="memory">Memory (MB)</Label>
                <Select
                  value={formData.memory}
                  onValueChange={(value) =>
                    setFormData({ ...formData, memory: value })
                  }
                >
                  <SelectTrigger id="memory">
                    <SelectValue placeholder="Select memory" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="128">128 MB</SelectItem>
                    <SelectItem value="256">256 MB</SelectItem>
                    <SelectItem value="512">512 MB</SelectItem>
                    <SelectItem value="1024">1024 MB</SelectItem>
                    <SelectItem value="2048">2048 MB</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="grid gap-3">
                <Label htmlFor="timeout">Timeout (seconds)</Label>
                <Select
                  value={formData.timeout}
                  onValueChange={(value) =>
                    setFormData({ ...formData, timeout: value })
                  }
                >
                  <SelectTrigger id="timeout">
                    <SelectValue placeholder="Select timeout" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="3">3 seconds</SelectItem>
                    <SelectItem value="10">10 seconds</SelectItem>
                    <SelectItem value="30">30 seconds</SelectItem>
                    <SelectItem value="60">60 seconds</SelectItem>
                    <SelectItem value="300">300 seconds</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
          </CardContent>

          <CardHeader className="border-t">
            <CardTitle>Function Code</CardTitle>
            <CardDescription>
              Write or upload your function code
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Tabs defaultValue="editor">
              <TabsList>
                <TabsTrigger value="editor">Code Editor</TabsTrigger>
                <TabsTrigger value="upload">Upload File</TabsTrigger>
              </TabsList>
              <TabsContent value="editor" className="mt-4">
                <FunctionCodeEditor />
              </TabsContent>
              <TabsContent value="upload" className="mt-4">
                <div className="flex items-center justify-center rounded-md border border-dashed p-12">
                  <div className="text-center">
                    <p className="text-sm text-muted-foreground">
                      Drag and drop your function code file here, or click to
                      browse
                    </p>
                    <Button variant="outline" className="mt-4">
                      Browse Files
                    </Button>
                  </div>
                </div>
              </TabsContent>
            </Tabs>
          </CardContent>

          <CardFooter className="flex justify-between border-t">
            <Button variant="outline" asChild>
              <Link href="/functions">Cancel</Link>
            </Button>
            <Button type="submit">Create Function</Button>
          </CardFooter>
        </Card>
      </form>
    </div>
  );
}
