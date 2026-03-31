"use client"

import { useEffect, useState } from "react"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { editServerV0, type ServerJson, type ServerResponse } from "@/lib/admin-api"
import { Loader2, Plus, Trash2 } from "lucide-react"
import { toast } from "sonner"

interface EditServerDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  server: ServerResponse | null
  onServerUpdated: () => void
}

const repositoryHosts = {
  github: "github.com",
  gitlab: "gitlab.com",
  bitbucket: "bitbucket.org",
} as const

type RepositorySource = keyof typeof repositoryHosts

type PackageForm = {
  identifier: string
  version: string
  registryType: string
  transport: string
  transportUrl: string
}

type RemoteForm = {
  type: string
  url: string
}

function validateRepositoryUrl(source: RepositorySource, rawUrl: string): string | null {
  const trimmedUrl = rawUrl.trim()
  if (!trimmedUrl) {
    return null
  }

  let parsedUrl: URL
  try {
    parsedUrl = new URL(trimmedUrl)
  } catch {
    return "Repository URL must be a valid absolute URL"
  }

  if (parsedUrl.protocol !== "http:" && parsedUrl.protocol !== "https:") {
    return "Repository URL must use http or https"
  }

  const expectedHost = repositoryHosts[source]
  if (parsedUrl.hostname !== expectedHost) {
    return `Repository URL must match the selected provider (${expectedHost})`
  }

  return null
}

function extractErrorMessage(error: unknown): string {
  if (error instanceof Error && error.message) {
    return error.message
  }

  if (typeof error === "string" && error.trim()) {
    return error
  }

  if (error && typeof error === "object") {
    const record = error as Record<string, unknown>
    const errors = Array.isArray(record.errors) ? record.errors : null
    if (errors) {
      for (const item of errors) {
        if (item && typeof item === "object") {
          const message = typeof (item as Record<string, unknown>).message === "string"
            ? (item as Record<string, unknown>).message as string
            : null
          if (message) {
            return message
          }
        }
      }
    }

    const detail = typeof record.detail === "string" ? record.detail : null
    if (detail) return detail
    const message = typeof record.message === "string" ? record.message : null
    if (message) return message
    const title = typeof record.title === "string" ? record.title : null
    if (title) return title

    const nestedError = record.error
    if (nestedError) return extractErrorMessage(nestedError)
  }

  return "Failed to update server"
}

const transportRequiresUrl = (transportType: string) =>
  transportType === "sse" || transportType === "streamable-http"

function toRepositorySource(source: string | undefined): RepositorySource {
  if (source === "gitlab" || source === "bitbucket") {
    return source
  }
  return "github"
}

function toPackageForms(server: ServerJson): PackageForm[] {
  if (!server.packages || server.packages.length === 0) {
    return []
  }

  return server.packages.map((pkg) => ({
    identifier: pkg.identifier ?? "",
    version: pkg.version ?? "",
    registryType: pkg.registryType ?? "oci",
    transport: pkg.transport?.type ?? "stdio",
    transportUrl: pkg.transport?.url ?? "",
  }))
}

function toRemoteForms(server: ServerJson): RemoteForm[] {
  if (!server.remotes || server.remotes.length === 0) {
    return []
  }
  return server.remotes.map((remote) => ({
    type: remote.type ?? "sse",
    url: remote.url ?? "",
  }))
}

export function EditServerDialog({ open, onOpenChange, server, onServerUpdated }: EditServerDialogProps) {
  const [loading, setLoading] = useState(false)

  const [schema, setSchema] = useState("https://static.modelcontextprotocol.io/schemas/2025-10-17/server.schema.json")
  const [name, setName] = useState("")
  const [title, setTitle] = useState("")
  const [description, setDescription] = useState("")
  const [version, setVersion] = useState("")
  const [websiteUrl, setWebsiteUrl] = useState("")
  const [repositorySource, setRepositorySource] = useState<RepositorySource>("github")
  const [repositoryUrl, setRepositoryUrl] = useState("")
  const [packages, setPackages] = useState<PackageForm[]>([])
  const [remotes, setRemotes] = useState<RemoteForm[]>([])

  useEffect(() => {
    if (!open || !server) {
      return
    }

    const current = server.server
    setSchema(current.$schema || "https://static.modelcontextprotocol.io/schemas/2025-10-17/server.schema.json")
    setName(current.name || "")
    setTitle(current.title || "")
    setDescription(current.description || "")
    setVersion(current.version || "")
    setWebsiteUrl(current.websiteUrl || "")
    setRepositorySource(toRepositorySource(current.repository?.source))
    setRepositoryUrl(current.repository?.url || "")
    setPackages(toPackageForms(current))
    setRemotes(toRemoteForms(current))
  }, [open, server])

  const addPackage = () => {
    setPackages([...packages, { identifier: "", version: "", registryType: "oci", transport: "stdio", transportUrl: "" }])
  }

  const removePackage = (index: number) => {
    setPackages(packages.filter((_, i) => i !== index))
  }

  const updatePackage = (index: number, field: keyof PackageForm, value: string) => {
    const updated = [...packages]
    updated[index] = { ...updated[index], [field]: value }
    setPackages(updated)
  }

  const addRemote = () => {
    setRemotes([...remotes, { type: "sse", url: "" }])
  }

  const removeRemote = (index: number) => {
    setRemotes(remotes.filter((_, i) => i !== index))
  }

  const updateRemote = (index: number, field: keyof RemoteForm, value: string) => {
    const updated = [...remotes]
    updated[index] = { ...updated[index], [field]: value }
    setRemotes(updated)
  }

  const handleSubmit = async () => {
    if (!server) {
      return
    }

    setLoading(true)
    try {
      const repositoryUrlError = validateRepositoryUrl(repositorySource, repositoryUrl)
      if (repositoryUrlError) {
        throw new Error(repositoryUrlError)
      }

      const filteredPackages = packages.filter((p) => p.identifier.trim() && p.version.trim())
      for (const p of filteredPackages) {
        const transportType = (p.transport || "stdio").trim()
        if (transportRequiresUrl(transportType) && !p.transportUrl.trim()) {
          throw new Error(`Package transport URL is required for ${transportType}`)
        }
      }

      const filteredRemotes = remotes.filter((r) => r.type.trim())
      for (const r of filteredRemotes) {
        const remoteType = r.type.trim()
        if (transportRequiresUrl(remoteType) && !r.url.trim()) {
          throw new Error(`Remote URL is required for ${remoteType}`)
        }
      }

      const updatedServer: ServerJson = {
        ...server.server,
        $schema: schema.trim(),
        name: name.trim(),
        description: description.trim(),
        version: version.trim(),
        title: title.trim() || undefined,
        websiteUrl: websiteUrl.trim() || undefined,
        repository: repositoryUrl.trim()
          ? {
            source: repositorySource,
            url: repositoryUrl.trim(),
          }
          : undefined,
        packages: filteredPackages.length > 0
          ? filteredPackages.map((p) => {
            const transportType = (p.transport || "stdio").trim()
            return {
              identifier: p.identifier.trim(),
              version: p.version.trim(),
              registryType: p.registryType.trim(),
              transport: {
                type: transportType,
                ...(transportRequiresUrl(transportType) ? { url: p.transportUrl.trim() } : {}),
              },
            }
          })
          : undefined,
        remotes: filteredRemotes.length > 0
          ? filteredRemotes.map((r) => {
            const remoteType = r.type.trim()
            return {
              type: remoteType,
              ...(transportRequiresUrl(remoteType) ? { url: r.url.trim() } : { url: r.url.trim() || undefined }),
            }
          })
          : undefined,
      }

      await editServerV0({
        path: {
          serverName: server.server.name,
          version: server.server.version,
        },
        body: updatedServer,
        throwOnError: true,
      })

      toast.success(`Server "${updatedServer.name}" updated successfully!`)
      onOpenChange(false)
      onServerUpdated()
    } catch (err) {
      toast.error(extractErrorMessage(err))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-6xl max-h-[90vh] overflow-y-auto px-8">
        <DialogHeader>
          <DialogTitle>Edit MCP Server</DialogTitle>
          <DialogDescription>
            Update MCP server metadata in your catalog
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-4">
          <div className="grid grid-cols-3 gap-4">
            <div className="space-y-2">
              <Label htmlFor="name">Server Name *</Label>
              <Input
                id="name"
                placeholder="io.example/my-server"
                value={name}
                disabled
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="title">Display Title</Label>
              <Input
                id="title"
                placeholder="My Server"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                disabled={loading}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="version">Version *</Label>
              <Input
                id="version"
                placeholder="1.0.0"
                value={version}
                disabled
              />
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="description">Description *</Label>
            <Textarea
              id="description"
              placeholder="Describe what this server does..."
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={3}
              disabled={loading}
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="websiteUrl">Website URL</Label>
              <Input
                id="websiteUrl"
                placeholder="https://example.com"
                value={websiteUrl}
                onChange={(e) => setWebsiteUrl(e.target.value)}
                disabled={loading}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="repositoryUrl">Repository URL</Label>
              <div className="flex gap-2">
                <Select value={repositorySource} onValueChange={(v) => setRepositorySource(v as RepositorySource)} disabled={loading}>
                  <SelectTrigger className="w-[120px]">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="github">GitHub</SelectItem>
                    <SelectItem value="gitlab">GitLab</SelectItem>
                    <SelectItem value="bitbucket">Bitbucket</SelectItem>
                  </SelectContent>
                </Select>
                <Input
                  id="repositoryUrl"
                  placeholder="https://github.com/user/repo"
                  value={repositoryUrl}
                  onChange={(e) => setRepositoryUrl(e.target.value)}
                  disabled={loading}
                  className="flex-1"
                />
              </div>
              <p className="text-xs text-muted-foreground">
                Repository URL must match the selected provider.
              </p>
            </div>
          </div>

          <div className="space-y-4 p-4 border rounded-lg">
            <div className="flex items-center justify-between">
              <h3 className="font-semibold text-sm">Packages</h3>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={addPackage}
                disabled={loading}
              >
                <Plus className="h-4 w-4 mr-1" />
                Add Package
              </Button>
            </div>

            {packages.map((pkg, index) => (
              <div key={index} className="space-y-2 p-3 border rounded-md">
                <div className="flex gap-2 items-start">
                  <Input
                    placeholder="Package identifier"
                    value={pkg.identifier}
                    onChange={(e) => updatePackage(index, "identifier", e.target.value)}
                    disabled={loading}
                    className="flex-1"
                  />
                  <Input
                    placeholder="Version"
                    value={pkg.version}
                    onChange={(e) => updatePackage(index, "version", e.target.value)}
                    disabled={loading}
                    className="w-32"
                  />
                  <select
                    value={pkg.registryType}
                    onChange={(e) => updatePackage(index, "registryType", e.target.value)}
                    className="px-3 py-2 border rounded-md bg-background text-foreground border-input focus:outline-none focus:ring-2 focus:ring-ring"
                    disabled={loading}
                  >
                    <option value="oci">oci</option>
                    <option value="npm">npm</option>
                    <option value="pypi">pypi</option>
                    <option value="docker">docker</option>
                  </select>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    onClick={() => removePackage(index)}
                    disabled={loading}
                    aria-label={`Remove package ${index + 1}`}
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
                <div className="flex gap-3 items-center pl-2">
                  <Label className="text-sm text-muted-foreground">Transport *:</Label>
                  {["stdio", "sse", "streamable-http"].map((transport) => (
                    <label key={transport} className="flex items-center gap-1.5 cursor-pointer">
                      <input
                        type="radio"
                        name={`transport-${index}`}
                        checked={pkg.transport === transport}
                        onChange={() => updatePackage(index, "transport", transport)}
                        disabled={loading}
                        className="border-gray-300"
                      />
                      <span className="text-sm">{transport}</span>
                    </label>
                  ))}
                </div>
                {transportRequiresUrl(pkg.transport) && (
                  <div className="pl-2">
                    <Input
                      placeholder="Transport URL (required) e.g. http://localhost:8080/mcp"
                      value={pkg.transportUrl}
                      onChange={(e) => updatePackage(index, "transportUrl", e.target.value)}
                      disabled={loading}
                      className="w-full"
                    />
                  </div>
                )}
              </div>
            ))}

            {packages.length === 0 && (
              <p className="text-sm text-muted-foreground text-center py-2">
                No packages added
              </p>
            )}
          </div>

          <div className="space-y-4 p-4 border rounded-lg">
            <div className="flex items-center justify-between">
              <h3 className="font-semibold text-sm">Remotes</h3>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={addRemote}
                disabled={loading}
              >
                <Plus className="h-4 w-4 mr-1" />
                Add Remote
              </Button>
            </div>

            {remotes.map((remote, index) => (
              <div key={index} className="flex gap-2 items-start">
                <select
                  value={remote.type}
                  onChange={(e) => updateRemote(index, "type", e.target.value)}
                  className="w-40 px-3 py-2 border rounded-md bg-background text-foreground border-input focus:outline-none focus:ring-2 focus:ring-ring"
                  disabled={loading}
                >
                  <option value="stdio">stdio</option>
                  <option value="sse">sse</option>
                  <option value="streamable-http">streamable-http</option>
                </select>
                <Input
                  placeholder={transportRequiresUrl(remote.type) ? "URL (required)" : "URL (optional)"}
                  value={remote.url}
                  onChange={(e) => updateRemote(index, "url", e.target.value)}
                  disabled={loading}
                  className="flex-1"
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  onClick={() => removeRemote(index)}
                  disabled={loading}
                  aria-label={`Remove remote ${index + 1}`}
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              </div>
            ))}

            {remotes.length === 0 && (
              <p className="text-sm text-muted-foreground text-center py-2">
                No remotes added
              </p>
            )}
          </div>
        </div>

        <div className="flex justify-end gap-2">
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={loading}
          >
            Cancel
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={loading || !name.trim() || !version.trim() || !description.trim()}
          >
            {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            Save Changes
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
