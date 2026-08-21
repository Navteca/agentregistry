"use client"

import { useState } from "react"
import { ServerResponse } from "@/lib/admin-api"
import { getSafeHttpUrl } from "@/lib/safe-url"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { RuntimeArgumentsTable } from "@/components/server-detail/runtime-arguments-table"
import { EnvironmentVariablesTable } from "@/components/server-detail/environment-variables-table"
import { ReviewSection } from "@/components/review-section"
import {
  Package,
  Calendar,
  ExternalLink,
  GitBranch,
  Globe,
  Code,
  Server,
  Link,
  Star,
  TrendingUp,
  Copy,
  ArrowLeft,
  History,
  Check,
  Shield,
  GitFork,
  Eye,
  Zap,
  CheckCircle,
  Clock,
  ShieldCheck,
  BadgeCheck,
} from "lucide-react"

interface ServerDetailProps {
  server: ServerResponse & { allVersions?: ServerResponse[] }
  onServerCopied?: () => void
  onReviewSubmitted?: () => void | Promise<void>
}

export function ServerDetail({ server, onServerCopied, onReviewSubmitted }: ServerDetailProps) {
  const [activeTab, setActiveTab] = useState("overview")
  const [selectedVersionNumber, setSelectedVersionNumber] = useState(server.server.version)
  const [jsonCopied, setJsonCopied] = useState(false)

  const allVersions = server.allVersions || [server]
  const selectedVersion = allVersions.find((version) => version.server.version === selectedVersionNumber) || server

  const { server: serverData, _meta } = selectedVersion
  const official = _meta?.['io.modelcontextprotocol.registry/official']
  const ownership = _meta?.['aregistry.ai/ownership']
  const registeredBy = ownership?.displayName || ownership?.subject

  const publisherProvided = serverData._meta?.['io.modelcontextprotocol.registry/publisher-provided'] as Record<string, unknown> | undefined
  const publisherMetadata = publisherProvided?.['aregistry.ai/metadata'] as Record<string, any> | undefined
  const githubStars = publisherMetadata?.stars as number | undefined
  const overallScore = publisherMetadata?.score as number | undefined
  const openSSFScore = (publisherMetadata?.scorecard as Record<string, any>)?.openssf as number | undefined
  const repoData = publisherMetadata?.repo as Record<string, any> | undefined
  const endpointHealth = publisherMetadata?.endpoint_health as Record<string, any> | undefined
  const scanData = publisherMetadata?.scans as Record<string, any> | undefined
  const identityData = publisherMetadata?.identity as Record<string, any> | undefined
  const securityScanning = publisherMetadata?.security_scanning as Record<string, any> | undefined
  const mcpScoring = publisherMetadata?.mcp_scoring as Record<string, any> | undefined

  const icon = serverData.icons?.[0]
  const safeIconSrc = getSafeHttpUrl(icon?.src)
  const safeWebsiteUrl = getSafeHttpUrl(serverData.websiteUrl)
  const safeRepositoryUrl = getSafeHttpUrl(serverData.repository?.url)

  const handleVersionChange = (version: string) => {
    const newVersion = allVersions.find(v => v.server.version === version)
    if (newVersion) setSelectedVersionNumber(newVersion.server.version)
  }

  const handleCopyJson = async () => {
    try {
      await navigator.clipboard.writeText(JSON.stringify(selectedVersion, null, 2))
      setJsonCopied(true)
      setTimeout(() => setJsonCopied(false), 2000)
    } catch (err) {
      console.error('Failed to copy JSON:', err)
    }
  }

  const formatDate = (dateString: string) => {
    try {
      return new Date(dateString).toLocaleString('en-US', {
        year: 'numeric',
        month: 'long',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      })
    } catch {
      return dateString
    }
  }

  return (
    <TooltipProvider>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex items-start gap-4">
          {safeIconSrc && (
            <img src={safeIconSrc} alt="" className="w-12 h-12 rounded flex-shrink-0" />
          )}
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 mb-1">
              <h1 className="text-2xl font-bold truncate">{serverData.title || serverData.name}</h1>
              {identityData?.org_is_verified && (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <ShieldCheck className="h-5 w-5 text-blue-500 flex-shrink-0" />
                  </TooltipTrigger>
                  <TooltipContent><p>Verified Organization</p></TooltipContent>
                </Tooltip>
              )}
              {identityData?.publisher_identity_verified_by_jwt && (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <BadgeCheck className="h-5 w-5 text-green-500 flex-shrink-0" />
                  </TooltipTrigger>
                  <TooltipContent><p>Verified Publisher</p></TooltipContent>
                </Tooltip>
              )}
            </div>
            <p className="text-[15px] text-muted-foreground">{serverData.name}</p>
          </div>
        </div>

        {/* Version selector */}
        {allVersions.length > 1 && (
          <div className="flex items-center gap-3 px-3 py-2 bg-accent/50 border border-primary/10 rounded-md">
            <History className="h-4 w-4 text-muted-foreground" />
            <span className="text-sm">{allVersions.length} versions</span>
            <Select value={selectedVersion.server.version} onValueChange={handleVersionChange}>
              <SelectTrigger className="w-[160px] h-7 text-sm">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {allVersions.map((version) => (
                  <SelectItem key={version.server.version} value={version.server.version}>
                    {version.server.version}
                    {version.server.version === server.server.version && " (latest)"}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}

        {/* Quick info pills */}
        <div className="flex flex-wrap gap-2 text-sm">
          <span className="flex items-center gap-1.5 px-2.5 py-1 bg-muted rounded text-sm">
            <span className="font-mono">{serverData.version}</span>
            {allVersions.length > 1 && (
              <Badge variant="secondary" className="text-[10px] px-1 py-0 h-3.5">{allVersions.length} total</Badge>
            )}
          </span>
          {official?.publishedAt && (
            <span className="flex items-center gap-1.5 px-2.5 py-1 bg-muted rounded text-sm">
              <Calendar className="h-3 w-3 text-muted-foreground" />
              {formatDate(official.publishedAt)}
            </span>
          )}
          <span className="flex items-center gap-1.5 px-2.5 py-1 bg-muted rounded text-sm">
            <span className="text-muted-foreground">Registered by</span>
            {registeredBy || <span className="text-muted-foreground">Unknown</span>}
          </span>
          {official?.updatedAt && (
            <span className="flex items-center gap-1.5 px-2.5 py-1 bg-muted rounded text-sm">
              <span className="text-muted-foreground">Last modified</span>
              {formatDate(official.updatedAt)}
            </span>
          )}
          {serverData.websiteUrl && (
            safeWebsiteUrl ? (
              <a
                href={safeWebsiteUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-1.5 px-2.5 py-1 bg-muted rounded text-sm hover:bg-muted/80 transition-colors text-primary"
              >
                <Globe className="h-3 w-3" />
                Website
                <ExternalLink className="h-2.5 w-2.5" />
              </a>
            ) : (
              <span className="flex items-center gap-1.5 px-2.5 py-1 bg-muted rounded text-sm">
                <Globe className="h-3 w-3" />
                {serverData.websiteUrl}
              </span>
            )
          )}
        </div>

        <ReviewSection
          artifactType="server"
          artifactName={serverData.name}
          artifactVersion={serverData.version}
          reviewSummary={_meta?.['aregistry.ai/review']}
          capabilities={_meta?.['aregistry.ai/capabilities']}
          onReviewSubmitted={onReviewSubmitted}
        />

        {/* Tabs */}
        <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
          <TabsList className="mb-4">
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="score">Score</TabsTrigger>
            {serverData.packages && serverData.packages.length > 0 && (
              <TabsTrigger value="packages">Packages</TabsTrigger>
            )}
            {serverData.remotes && serverData.remotes.length > 0 && (
              <TabsTrigger value="remotes">Remotes</TabsTrigger>
            )}
            <TabsTrigger value="raw">Raw</TabsTrigger>
          </TabsList>

          <TabsContent value="overview" className="space-y-6">
            <section>
              <h3 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground mb-2">Description</h3>
              <p className="text-[15px] leading-relaxed">{serverData.description}</p>
            </section>

            {serverData.repository?.url && (
              <section>
                <h3 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground mb-2">Repository</h3>
                <div className="space-y-2 text-sm">
                  {serverData.repository.source && (
                    <div className="flex items-center justify-between">
                      <span className="text-muted-foreground">Source</span>
                      <Badge variant="outline" className="text-xs">{serverData.repository.source}</Badge>
                    </div>
                  )}
                  <div className="flex items-center justify-between">
                    <span className="text-muted-foreground">URL</span>
                    {safeRepositoryUrl ? (
                      <a
                        href={safeRepositoryUrl}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-sm text-primary hover:underline flex items-center gap-1"
                      >
                        {serverData.repository.url} <ExternalLink className="h-3 w-3" />
                      </a>
                    ) : (
                      <span className="text-sm break-all">{serverData.repository.url}</span>
                    )}
                  </div>
                </div>
              </section>
            )}
          </TabsContent>

          <TabsContent value="score" className="space-y-6">
            {/* Score cards */}
            {(overallScore !== undefined || openSSFScore !== undefined) && (
              <div className="grid grid-cols-2 gap-4">
                {overallScore !== undefined && (
                  <div className="p-4 rounded-lg border bg-muted/30">
                    <div className="flex items-center gap-3">
                      <TrendingUp className="h-5 w-5 text-primary" />
                      <div>
                        <p className="text-xs text-muted-foreground">Overall Score</p>
                        <p className="text-2xl font-bold">{overallScore.toFixed(2)}</p>
                      </div>
                    </div>
                  </div>
                )}
                {openSSFScore !== undefined && (
                  <div className="p-4 rounded-lg border bg-muted/30">
                    <div className="flex items-center gap-3">
                      <Shield className="h-5 w-5 text-green-500" />
                      <div>
                        <p className="text-xs text-muted-foreground">OpenSSF Scorecard</p>
                        <p className="text-2xl font-bold">{openSSFScore.toFixed(1)}/10</p>
                      </div>
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* Repo stats */}
            {(githubStars !== undefined || repoData) && (
              <section>
                <h3 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground mb-3">Repository Stats</h3>
                <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                  {githubStars !== undefined && (
                    <div className="flex items-center gap-2 p-3 rounded-md bg-muted/50">
                      <Star className="h-4 w-4 text-amber-500 fill-amber-500" />
                      <div>
                        <p className="text-[10px] text-muted-foreground">Stars</p>
                        <p className="text-lg font-bold">{githubStars.toLocaleString()}</p>
                      </div>
                    </div>
                  )}
                  {repoData?.forks_count !== undefined && (
                    <div className="flex items-center gap-2 p-3 rounded-md bg-muted/50">
                      <GitFork className="h-4 w-4 text-muted-foreground" />
                      <div>
                        <p className="text-[10px] text-muted-foreground">Forks</p>
                        <p className="text-lg font-bold">{repoData.forks_count.toLocaleString()}</p>
                      </div>
                    </div>
                  )}
                  {repoData?.watchers_count !== undefined && (
                    <div className="flex items-center gap-2 p-3 rounded-md bg-muted/50">
                      <Eye className="h-4 w-4 text-muted-foreground" />
                      <div>
                        <p className="text-[10px] text-muted-foreground">Watchers</p>
                        <p className="text-lg font-bold">{repoData.watchers_count.toLocaleString()}</p>
                      </div>
                    </div>
                  )}
                  {repoData?.primary_language && (
                    <div className="flex items-center gap-2 p-3 rounded-md bg-muted/50">
                      <Code className="h-4 w-4 text-muted-foreground" />
                      <div>
                        <p className="text-[10px] text-muted-foreground">Language</p>
                        <p className="text-sm font-bold">{repoData.primary_language}</p>
                      </div>
                    </div>
                  )}
                </div>
                {serverData.repository?.url && (
                  safeRepositoryUrl ? (
                    <a
                      href={safeRepositoryUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="flex items-center gap-1.5 text-xs text-primary hover:underline mt-3"
                    >
                      <ExternalLink className="h-3 w-3" />
                      View Repository
                    </a>
                  ) : (
                    <span className="text-xs break-all">{serverData.repository.url}</span>
                  )
                )}
              </section>
            )}

            {/* Endpoint Health */}
            {endpointHealth && (
              <section>
                <h3 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground mb-3">Endpoint Health</h3>
                <div className="grid grid-cols-3 gap-3">
                  <div className="flex items-center gap-2 p-3 rounded-md bg-muted/50">
                    <CheckCircle className={`h-4 w-4 ${endpointHealth.reachable ? 'text-green-500' : 'text-red-500'}`} />
                    <div>
                      <p className="text-[10px] text-muted-foreground">Status</p>
                      <p className="text-sm font-semibold">{endpointHealth.reachable ? 'Reachable' : 'Unreachable'}</p>
                    </div>
                  </div>
                  {endpointHealth.response_ms !== undefined && (
                    <div className="flex items-center gap-2 p-3 rounded-md bg-muted/50">
                      <Clock className="h-4 w-4 text-muted-foreground" />
                      <div>
                        <p className="text-[10px] text-muted-foreground">Response</p>
                        <p className="text-sm font-semibold">{endpointHealth.response_ms}ms</p>
                      </div>
                    </div>
                  )}
                  {endpointHealth.last_checked_at && (
                    <div className="flex items-center gap-2 p-3 rounded-md bg-muted/50">
                      <Calendar className="h-4 w-4 text-muted-foreground" />
                      <div>
                        <p className="text-[10px] text-muted-foreground">Last Checked</p>
                        <p className="text-sm font-semibold">{new Date(endpointHealth.last_checked_at).toLocaleDateString()}</p>
                      </div>
                    </div>
                  )}
                </div>
              </section>
            )}

            {/* Security */}
            {(scanData || securityScanning) && (
              <section>
                <h3 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground mb-3">Security</h3>

                {scanData?.dependency_health && (
                  <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
                    <div className="p-3 rounded-md bg-muted/50 text-center">
                      <p className="text-[10px] text-muted-foreground">Total Packages</p>
                      <p className="text-lg font-bold">{scanData.dependency_health.packages_total}</p>
                    </div>
                    <div className="p-3 rounded-md bg-muted/50 text-center">
                      <p className="text-[10px] text-muted-foreground">Copyleft</p>
                      <p className="text-lg font-bold">{scanData.dependency_health.copyleft_licenses}</p>
                    </div>
                    <div className="p-3 rounded-md bg-muted/50 text-center">
                      <p className="text-[10px] text-muted-foreground">Unknown Licenses</p>
                      <p className="text-lg font-bold">{scanData.dependency_health.unknown_licenses}</p>
                    </div>
                    {scanData.dependency_health.ecosystems && (
                      <div className="p-3 rounded-md bg-muted/50 text-center">
                        <p className="text-[10px] text-muted-foreground">Ecosystems</p>
                        <div className="text-xs font-semibold space-y-0.5 mt-1">
                          {Object.entries(scanData.dependency_health.ecosystems).map(([key, value]) => (
                            <div key={key}>{key}: {String(value)}</div>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                )}

                {securityScanning && (
                  <div className="flex items-center gap-3 mb-4">
                    <div className="flex items-center gap-1.5 text-sm">
                      <CheckCircle className={`h-3.5 w-3.5 ${securityScanning.codeql_enabled ? 'text-green-500' : 'text-muted-foreground/40'}`} />
                      CodeQL
                    </div>
                    <div className="flex items-center gap-1.5 text-sm">
                      <CheckCircle className={`h-3.5 w-3.5 ${securityScanning.dependabot_enabled ? 'text-green-500' : 'text-muted-foreground/40'}`} />
                      Dependabot
                    </div>
                  </div>
                )}

                {scanData?.details && scanData.details.length > 0 && (
                  <div className="space-y-1">
                    {scanData.details.map((detail: string, idx: number) => (
                      <div key={idx} className="text-xs p-2 bg-muted rounded font-mono">{detail}</div>
                    ))}
                  </div>
                )}

                {scanData?.summary && (
                  <div className="mt-3 p-3 bg-accent/50 rounded-md border border-primary/10">
                    <p className="text-[10px] font-semibold text-muted-foreground mb-0.5">Summary</p>
                    <p className="text-xs font-mono">{scanData.summary}</p>
                  </div>
                )}
              </section>
            )}

            {/* MCP Scoring */}
            {mcpScoring && (
              <section>
                <h3 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground mb-3">MCP Scoring</h3>

                {/* Category scores */}
                {mcpScoring.scores && (
                  <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
                    <ScoreBadge label="Total" value={mcpScoring.scores.total} icon={<TrendingUp className="h-4 w-4" />} />
                    <ScoreBadge label="Security" value={mcpScoring.scores.security} icon={<Shield className="h-4 w-4" />} />
                    <ScoreBadge label="Best Practices" value={mcpScoring.scores['best-practices']} icon={<BadgeCheck className="h-4 w-4" />} />
                    <ScoreBadge label="Cost" value={mcpScoring.scores.cost} icon={<Zap className="h-4 w-4" />} />
                  </div>
                )}

                {/* Overall assessment */}
                {mcpScoring.analysis?.overall_assessment && (
                  <div className="p-3 bg-accent/50 rounded-md border border-primary/10 mb-4">
                    <p className="text-xs font-semibold">{mcpScoring.analysis.overall_assessment}</p>
                  </div>
                )}

                {/* MCP Surface */}
                {mcpScoring.mcp_surface && (
                  <div className="mb-4">
                    <h4 className="text-xs font-semibold text-muted-foreground mb-2">MCP Surface</h4>
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
                      <div className="p-2 rounded bg-muted/50 text-center">
                        <p className="text-[10px] text-muted-foreground">Tools</p>
                        <p className="text-lg font-bold">{mcpScoring.mcp_surface.tool_count}</p>
                      </div>
                      <div className="p-2 rounded bg-muted/50 text-center">
                        <p className="text-[10px] text-muted-foreground">Prompts</p>
                        <p className="text-lg font-bold">{mcpScoring.mcp_surface.prompt_count}</p>
                      </div>
                      <div className="p-2 rounded bg-muted/50 text-center">
                        <p className="text-[10px] text-muted-foreground">Resources</p>
                        <p className="text-lg font-bold">{mcpScoring.mcp_surface.resource_count}</p>
                      </div>
                      {mcpScoring.mcp_surface.transports && (
                        <div className="p-2 rounded bg-muted/50 text-center">
                          <p className="text-[10px] text-muted-foreground">Transports</p>
                          <div className="flex flex-wrap gap-1 justify-center mt-1">
                            {(mcpScoring.mcp_surface.transports as string[]).map((t: string) => (
                              <Badge key={t} variant="secondary" className="text-[10px]">{t}</Badge>
                            ))}
                          </div>
                        </div>
                      )}
                    </div>
                    {mcpScoring.mcp_surface.tool_names?.length > 0 && (
                      <div className="flex flex-wrap gap-1 mt-2">
                        {(mcpScoring.mcp_surface.tool_names as string[]).map((name: string) => (
                          <Badge key={name} variant="outline" className="text-[10px] font-mono">{name}</Badge>
                        ))}
                      </div>
                    )}
                  </div>
                )}

                {/* Key strengths */}
                {mcpScoring.analysis?.key_strengths?.length > 0 && (
                  <div className="mb-4">
                    <h4 className="text-xs font-semibold text-muted-foreground mb-2">Key Strengths</h4>
                    <div className="space-y-1">
                      {(mcpScoring.analysis.key_strengths as string[]).map((s: string, i: number) => (
                        <div key={i} className="flex items-start gap-2 text-xs">
                          <CheckCircle className="h-3.5 w-3.5 text-green-500 mt-0.5 shrink-0" />
                          <span>{s}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {/* Critical issues */}
                {mcpScoring.analysis?.critical_issues?.length > 0 && (
                  <div className="mb-4">
                    <h4 className="text-xs font-semibold text-muted-foreground mb-2">Issues</h4>
                    <div className="space-y-2">
                      {(mcpScoring.analysis.critical_issues as Array<Record<string, any>>).map((issue, i: number) => (
                        <div key={i} className="p-2 rounded bg-red-500/5 border border-red-500/20 text-xs">
                          <div className="flex items-center gap-2">
                            <Badge variant="destructive" className="text-[10px]">{issue.severity}</Badge>
                            <span className="font-mono text-[10px] text-muted-foreground">{issue.rule_id}</span>
                          </div>
                          <p className="mt-1">{issue.summary}</p>
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {/* Rules detail (collapsible) */}
                {mcpScoring.rules?.length > 0 && (
                  <details className="mb-4">
                    <summary className="text-xs font-semibold text-muted-foreground cursor-pointer mb-2 select-none">
                      Rules ({(mcpScoring.rules as Array<Record<string, any>>).filter((r) => r.outcome === 'met').length}/{(mcpScoring.rules as Array<Record<string, any>>).length} met)
                    </summary>
                    <div className="space-y-1 max-h-64 overflow-y-auto">
                      {(mcpScoring.rules as Array<Record<string, any>>).map((rule, i: number) => (
                        <div key={i} className="flex items-center gap-2 text-xs p-1.5 rounded bg-muted/30">
                          <RuleOutcomeIcon outcome={rule.outcome} />
                          <span className="font-mono text-[10px] text-muted-foreground w-44 shrink-0">{rule.rule_id}</span>
                          <span className="truncate" title={rule.rationale}>{rule.rationale}</span>
                        </div>
                      ))}
                    </div>
                  </details>
                )}

                {/* Scored at */}
                {mcpScoring.scored_at && (
                  <p className="text-[10px] text-muted-foreground mt-2">
                    Scored at {new Date(mcpScoring.scored_at as string).toLocaleString()}
                  </p>
                )}
              </section>
            )}

            {!publisherMetadata && !mcpScoring && (
              <div className="text-center py-12">
                <TrendingUp className="h-8 w-8 mx-auto mb-3 text-muted-foreground opacity-40" />
                <p className="text-sm text-muted-foreground">No scoring data available</p>
                <p className="text-xs text-muted-foreground mt-1">Data will be fetched on next import/refresh</p>
              </div>
            )}
          </TabsContent>

          <TabsContent value="packages" className="space-y-4">
            {serverData.packages && serverData.packages.length > 0 ? (
              <div className="space-y-4">
                {serverData.packages.map((pkg, i) => (
                  <div key={i} className="p-4 rounded-lg border">
                    <div className="flex items-center justify-between mb-3">
                      <div className="flex items-center gap-2">
                        <Package className="h-4 w-4 text-primary" />
                        <h4 className="text-sm font-semibold">{pkg.identifier}</h4>
                      </div>
                      <Badge variant="outline" className="text-xs">{pkg.registryType}</Badge>
                    </div>
                    <div className="space-y-1.5 text-sm mb-3 pb-3 border-b">
                      <div className="flex justify-between text-xs">
                        <span className="text-muted-foreground">Version</span>
                        <span className="font-mono">{pkg.version}</span>
                      </div>
                      {(pkg as any).runtimeHint && (
                        <div className="flex justify-between text-xs">
                          <span className="text-muted-foreground">Runtime</span>
                          <Badge variant="secondary" className="text-[10px] h-4">{(pkg as any).runtimeHint}</Badge>
                        </div>
                      )}
                      {(pkg as any).transport?.type && (
                        <div className="flex justify-between text-xs">
                          <span className="text-muted-foreground">Transport</span>
                          <Badge variant="secondary" className="text-[10px] h-4">{(pkg as any).transport.type}</Badge>
                        </div>
                      )}
                    </div>
                    <RuntimeArgumentsTable arguments={(pkg as any).runtimeArguments} />
                    <EnvironmentVariablesTable variables={(pkg as any).environmentVariables} />
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-center text-sm text-muted-foreground py-8">No packages defined</p>
            )}
          </TabsContent>

          <TabsContent value="remotes" className="space-y-3">
            {serverData.remotes && serverData.remotes.length > 0 ? (
              <div className="space-y-3">
                {serverData.remotes.map((remote, i) => {
                  const safeRemoteUrl = getSafeHttpUrl(remote.url)
                  return (
                    <div key={i} className="p-4 rounded-lg border">
                      <div className="flex items-center justify-between mb-2">
                        <div className="flex items-center gap-2">
                          <Server className="h-4 w-4 text-primary" />
                          <h4 className="text-sm font-semibold">Remote {i + 1}</h4>
                        </div>
                        <Badge variant="outline" className="text-xs">{remote.type}</Badge>
                      </div>
                      {remote.url && (
                        <div className="flex items-center gap-2 text-sm">
                          <Link className="h-3.5 w-3.5 text-muted-foreground" />
                          {safeRemoteUrl ? (
                            <a
                              href={safeRemoteUrl}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="text-primary hover:underline break-all text-xs"
                            >
                              {remote.url}
                            </a>
                          ) : (
                            <span className="break-all text-xs">{remote.url}</span>
                          )}
                        </div>
                      )}
                    </div>
                  )
                })}
              </div>
            ) : (
              <p className="text-center text-sm text-muted-foreground py-8">No remotes defined</p>
            )}
          </TabsContent>

          <TabsContent value="raw">
            <div className="rounded-lg border p-4">
              <div className="flex items-center justify-between mb-3">
                <h3 className="text-sm font-semibold flex items-center gap-2">
                  <Code className="h-4 w-4" />
                  Raw JSON
                </h3>
                <Button variant="outline" size="sm" onClick={handleCopyJson} className="gap-1.5 h-7 text-xs">
                  {jsonCopied ? <><Check className="h-3 w-3" /> Copied</> : <><Copy className="h-3 w-3" /> Copy</>}
                </Button>
              </div>
              <pre className="bg-muted p-3 rounded-md overflow-x-auto text-xs leading-relaxed">
                {JSON.stringify(selectedVersion, null, 2)}
              </pre>
            </div>
          </TabsContent>
        </Tabs>
      </div>
    </TooltipProvider>
  )
}

function scoreColorClass(value: number): string {
  if (value >= 90) return "text-green-500"
  if (value >= 70) return "text-yellow-500"
  return "text-red-500"
}

function ScoreBadge({ label, value, icon }: { label: string; value: number | undefined; icon: React.ReactNode }) {
  if (value === undefined) return null
  return (
    <div className="p-3 rounded-md bg-muted/50 text-center">
      <div className={`flex justify-center mb-1 ${scoreColorClass(value)}`}>{icon}</div>
      <p className="text-[10px] text-muted-foreground">{label}</p>
      <p className={`text-2xl font-bold ${scoreColorClass(value)}`}>{value}</p>
    </div>
  )
}

function RuleOutcomeIcon({ outcome }: { outcome: string }) {
  if (outcome === "met") return <CheckCircle className="h-3.5 w-3.5 text-green-500 shrink-0" />
  if (outcome === "not_met") return <ShieldCheck className="h-3.5 w-3.5 text-red-500 shrink-0" />
  return <Clock className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
}
