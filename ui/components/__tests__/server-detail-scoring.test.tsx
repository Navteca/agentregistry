import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, it, expect } from "vitest"
import { ServerDetail as ServerDetailComponent } from "../server-detail"
import type { ServerResponse } from "@/lib/api/types.gen"
import { FrontendConfigProvider, type FrontendConfig } from "@/lib/frontend-config"

const frontendConfig: FrontendConfig = {
  keycloak_url: "",
  keycloak_realm: "",
  keycloak_client_id: "",
  anonymous_auth_enabled: true,
  show_github_link: true,
  show_discord_link: true,
  review_types: ["security"],
  review_outcomes: ["pass", "fail"],
  review_failure_outcome: "fail",
  review_override_outcome: "override",
}

type ServerDetailProps = Parameters<typeof ServerDetailComponent>[0]

function ServerDetail(props: ServerDetailProps) {
  return (
    <FrontendConfigProvider config={frontendConfig}>
      <ServerDetailComponent {...props} />
    </FrontendConfigProvider>
  )
}

function buildMockServer(overrides?: Partial<ServerResponse>): ServerResponse & { allVersions?: ServerResponse[] } {
  return {
    server: {
      $schema: "https://modelcontextprotocol.io/schemas/server.json",
      name: "io.navteca/hello-mcp",
      description: "Hello MCP server",
      version: "1.0.0",
      repository: { url: "https://github.com/navteca/hello-mcp", source: "github" },
    },
    _meta: {
      "io.modelcontextprotocol.registry/official": {
        publishedAt: "2024-11-01T00:00:00Z",
        updatedAt: "2025-08-20T00:00:00Z",
        status: "active",
        isLatest: true,
      },
    },
    ...overrides,
  }
}

function buildMockServerWithScoring(): ServerResponse & { allVersions?: ServerResponse[] } {
  return buildMockServer({
    server: {
      $schema: "https://modelcontextprotocol.io/schemas/server.json",
      name: "io.navteca/hello-mcp",
      description: "Hello MCP server",
      version: "1.0.0",
      repository: { url: "https://github.com/navteca/hello-mcp", source: "github" },
      _meta: {
        "io.modelcontextprotocol.registry/publisher-provided": {
          "aregistry.ai/metadata": {
            stars: 42,
            score: 3.14,
            mcp_scoring: {
              scores: { security: 91, "best-practices": 88, cost: 100, total: 93 },
              rules: [
                { rule_id: "cost-01-minimal-dependencies", outcome: "met", evidence: "import logging", rationale: "Limited imports" },
                { rule_id: "sec-03-tool-shadowing", outcome: "not_met", evidence: "search_publications", rationale: "Generic name" },
                { rule_id: "int-04-predictable-behavior", outcome: "not_verifiable", evidence: "data = await", rationale: "External API" },
              ],
              mcp_surface: {
                tool_count: 2,
                tool_names: ["search_publications", "get_publication"],
                prompt_count: 0,
                resource_count: 0,
                transports: ["http", "sse", "stdio"],
              },
              summary: "MCP Server Analysis: Total Score 93/100",
              analysis: {
                overall_assessment: "Excellent (93/100): Production-ready",
                security: { score: 91, rules_met: 9, rules_not_met: 1 },
                "best-practices": { score: 88, rules_met: 3, rules_not_met: 0 },
                cost: { score: 100, rules_met: 6, rules_not_met: 0 },
                key_strengths: ["Protected against tool poisoning", "Well-documented tools"],
                critical_issues: [
                  { rule_id: "sec-03-tool-shadowing", category: "security", summary: "Generic tool name", severity: "medium" },
                ],
                recommendations: ["Continue following MCP best practices."],
              },
              scored_at: "2026-03-26T12:00:00Z",
            },
          },
        },
      },
    },
  })
}

describe("ServerDetail Score Tab - MCP Scoring", () => {
  it("renders the ownership display name instead of the subject", () => {
    const server = buildMockServer({
      _meta: {
        "io.modelcontextprotocol.registry/official": {
          publishedAt: "2024-11-01T00:00:00Z",
          updatedAt: "2025-08-20T00:00:00Z",
          status: "active",
          isLatest: true,
        },
        "aregistry.ai/ownership": {
          displayName: "Ada Lovelace",
          subject: "oidc-subject",
        },
      },
    })

    render(<ServerDetail server={server} />)

    expect(screen.getByText("Ada Lovelace")).toBeInTheDocument()
    expect(screen.queryByText("oidc-subject")).not.toBeInTheDocument()
  })

  it("falls back to the ownership subject when the display name is empty", () => {
    const server = buildMockServer({
      _meta: {
        "io.modelcontextprotocol.registry/official": {
          publishedAt: "2024-11-01T00:00:00Z",
          updatedAt: "2025-08-20T00:00:00Z",
          status: "active",
          isLatest: true,
        },
        "aregistry.ai/ownership": {
          displayName: "",
          subject: "github-user",
        },
      },
    })

    render(<ServerDetail server={server} />)

    expect(screen.getByText("github-user")).toBeInTheDocument()
  })

  it("renders a placeholder when ownership is absent", () => {
    render(<ServerDetail server={buildMockServer()} />)

    expect(screen.getByText("Unknown")).toBeInTheDocument()
  })

  it("does not render a last-modified badge when updatedAt is absent", () => {
    const server = buildMockServer({
      _meta: {
        "io.modelcontextprotocol.registry/official": {
          publishedAt: "2024-11-01T00:00:00Z",
          status: "active",
          isLatest: true,
        },
      },
    })

    render(<ServerDetail server={server} />)

    expect(screen.queryByText("Last modified")).not.toBeInTheDocument()
  })

  it("renders HTML in a display name as literal text", () => {
    const server = buildMockServer({
      _meta: {
        "io.modelcontextprotocol.registry/official": {
          publishedAt: "2024-11-01T00:00:00Z",
          updatedAt: "2025-08-20T00:00:00Z",
          status: "active",
          isLatest: true,
        },
        "aregistry.ai/ownership": {
          displayName: "<script>alert(1)</script>",
          subject: "oidc-subject",
        },
      },
    })

    const { container } = render(<ServerDetail server={server} />)

    expect(screen.getByText("<script>alert(1)</script>")).toBeInTheDocument()
    expect(container.querySelector("script")).not.toBeInTheDocument()
  })

  it("does not render a javascript website URL as a link", () => {
    const server = buildMockServer()
    server.server.websiteUrl = "javascript:alert(1)"

    render(<ServerDetail server={server} />)

    expect(screen.queryByRole("link", { name: "Website" })).not.toBeInTheDocument()
    expect(screen.getByText("javascript:alert(1)")).toBeInTheDocument()
  })

  it("renders category scores when mcp_scoring data exists", async () => {
    const user = userEvent.setup()
    render(<ServerDetail server={buildMockServerWithScoring()} />)

    await user.click(screen.getByRole("button", { name: "Score" }))

    expect(screen.getByText("MCP Scoring")).toBeInTheDocument()
    expect(screen.getByText("93")).toBeInTheDocument()
    expect(screen.getByText("91")).toBeInTheDocument()
    expect(screen.getByText("88")).toBeInTheDocument()
    expect(screen.getByText("100")).toBeInTheDocument()
  })

  it("renders overall assessment", async () => {
    const user = userEvent.setup()
    render(<ServerDetail server={buildMockServerWithScoring()} />)

    await user.click(screen.getByRole("button", { name: "Score" }))

    expect(screen.getByText("Excellent (93/100): Production-ready")).toBeInTheDocument()
  })

  it("renders MCP surface data", async () => {
    const user = userEvent.setup()
    render(<ServerDetail server={buildMockServerWithScoring()} />)

    await user.click(screen.getByRole("button", { name: "Score" }))

    expect(screen.getByText("MCP Surface")).toBeInTheDocument()
    expect(screen.getByText("2")).toBeInTheDocument() // tool_count
    expect(screen.getByText("search_publications")).toBeInTheDocument()
    expect(screen.getByText("get_publication")).toBeInTheDocument()
  })

  it("renders transports as badges", async () => {
    const user = userEvent.setup()
    render(<ServerDetail server={buildMockServerWithScoring()} />)

    await user.click(screen.getByRole("button", { name: "Score" }))

    expect(screen.getByText("http")).toBeInTheDocument()
    expect(screen.getByText("sse")).toBeInTheDocument()
    expect(screen.getByText("stdio")).toBeInTheDocument()
  })

  it("renders key strengths", async () => {
    const user = userEvent.setup()
    render(<ServerDetail server={buildMockServerWithScoring()} />)

    await user.click(screen.getByRole("button", { name: "Score" }))

    expect(screen.getByText("Key Strengths")).toBeInTheDocument()
    expect(screen.getByText("Protected against tool poisoning")).toBeInTheDocument()
    expect(screen.getByText("Well-documented tools")).toBeInTheDocument()
  })

  it("renders critical issues", async () => {
    const user = userEvent.setup()
    render(<ServerDetail server={buildMockServerWithScoring()} />)

    await user.click(screen.getByRole("button", { name: "Score" }))

    expect(screen.getByText("Issues")).toBeInTheDocument()
    expect(screen.getByText("Generic tool name")).toBeInTheDocument()
    expect(screen.getByText("medium")).toBeInTheDocument()
    expect(screen.getAllByText("sec-03-tool-shadowing").length).toBeGreaterThanOrEqual(1)
  })

  it("renders rules detail in collapsible section", async () => {
    const user = userEvent.setup()
    render(<ServerDetail server={buildMockServerWithScoring()} />)

    await user.click(screen.getByRole("button", { name: "Score" }))

    const rulesSummary = screen.getByText(/Rules \(1\/3 met\)/)
    expect(rulesSummary).toBeInTheDocument()

    expect(screen.getByText("cost-01-minimal-dependencies")).toBeInTheDocument()
    expect(screen.getByText("Limited imports")).toBeInTheDocument()
  })

  it("renders scored_at timestamp", async () => {
    const user = userEvent.setup()
    render(<ServerDetail server={buildMockServerWithScoring()} />)

    await user.click(screen.getByRole("button", { name: "Score" }))

    expect(screen.getByText(/Scored at/)).toBeInTheDocument()
  })

  it("shows empty state when no publisher metadata exists", async () => {
    const user = userEvent.setup()
    const server = buildMockServer()
    render(<ServerDetail server={server} />)

    await user.click(screen.getByRole("button", { name: "Score" }))

    expect(screen.getByText("No scoring data available")).toBeInTheDocument()
  })

  it("shows repo stats alongside mcp_scoring when both exist", async () => {
    const user = userEvent.setup()
    const server = buildMockServerWithScoring()
    render(<ServerDetail server={server} />)

    await user.click(screen.getByRole("button", { name: "Score" }))

    expect(screen.getByText("MCP Scoring")).toBeInTheDocument()
    expect(screen.getByText("42")).toBeInTheDocument() // stars
  })

  it("renders score color coding for high score (green)", async () => {
    const user = userEvent.setup()
    render(<ServerDetail server={buildMockServerWithScoring()} />)

    await user.click(screen.getByRole("button", { name: "Score" }))

    const totalScoreElement = screen.getByText("93")
    expect(totalScoreElement.className).toContain("text-green-500")
  })
})
