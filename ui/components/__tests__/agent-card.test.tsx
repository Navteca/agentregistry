import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, it, expect, vi } from "vitest"
import { AgentCard } from "../agent-card"
import type { AgentResponse } from "@/lib/api/types.gen"

const mockAgent: AgentResponse = {
  agent: {
    name: "test-agent",
    description: "A test agent for unit testing",
    version: "1.0.0",
    framework: "langchain",
    language: "python",
    image: "registry.example.com/test-agent:latest",
    modelProvider: "openai",
    modelName: "gpt-4",
  },
  _meta: {
    "io.modelcontextprotocol.registry/official": {
      publishedAt: "2025-01-15T00:00:00Z",
      updatedAt: "2025-01-15T00:00:00Z",
      status: "active",
      isLatest: true,
    },
  },
}

describe("AgentCard", () => {
  it("renders agent name and description", () => {
    render(<AgentCard agent={mockAgent} />)
    expect(screen.getByText("test-agent")).toBeInTheDocument()
    expect(screen.getByText("A test agent for unit testing")).toBeInTheDocument()
  })

  it("renders framework and language badges", () => {
    render(<AgentCard agent={mockAgent} />)
    expect(screen.getByText("langchain")).toBeInTheDocument()
    expect(screen.getByText("python")).toBeInTheDocument()
  })

  it("renders version", () => {
    render(<AgentCard agent={mockAgent} />)
    expect(screen.getByText("1.0.0")).toBeInTheDocument()
  })

  it("renders model provider and name", () => {
    render(<AgentCard agent={mockAgent} />)
    expect(screen.getByText("openai")).toBeInTheDocument()
    expect(screen.getByText("gpt-4")).toBeInTheDocument()
  })

  it("renders the ownership display name instead of the subject", () => {
    const ownedAgent: AgentResponse = {
      ...mockAgent,
      _meta: {
        ...mockAgent._meta,
        "aregistry.ai/ownership": {
          displayName: "Ada Lovelace",
          subject: "oidc-subject",
        },
      },
    }
    render(<AgentCard agent={ownedAgent} />)
    expect(screen.getByText("Ada Lovelace")).toBeInTheDocument()
    expect(screen.queryByText("oidc-subject")).not.toBeInTheDocument()
  })

  it("falls back to the ownership subject when the display name is empty", () => {
    const ownedAgent: AgentResponse = {
      ...mockAgent,
      _meta: {
        ...mockAgent._meta,
        "aregistry.ai/ownership": {
          displayName: "",
          subject: "github-user",
        },
      },
    }
    render(<AgentCard agent={ownedAgent} />)
    expect(screen.getByText("github-user")).toBeInTheDocument()
  })

  it("renders a placeholder when ownership is absent", () => {
    render(<AgentCard agent={mockAgent} />)
    expect(screen.getByText("Unknown")).toBeInTheDocument()
  })

  it("does not render a last-modified element when updatedAt is absent", () => {
    const agentWithoutUpdatedAt: AgentResponse = {
      ...mockAgent,
      _meta: {},
    }
    render(<AgentCard agent={agentWithoutUpdatedAt} />)
    expect(screen.queryByText("Last modified")).not.toBeInTheDocument()
  })

  it("renders HTML in a display name as literal text", () => {
    const ownedAgent: AgentResponse = {
      ...mockAgent,
      _meta: {
        ...mockAgent._meta,
        "aregistry.ai/ownership": {
          displayName: "<script>alert(1)</script>",
          subject: "oidc-subject",
        },
      },
    }
    const { container } = render(<AgentCard agent={ownedAgent} />)
    expect(screen.getByText("<script>alert(1)</script>")).toBeInTheDocument()
    expect(container.querySelector("script")).not.toBeInTheDocument()
  })

  it("does not render a javascript repository URL as a link", () => {
    const unsafeAgent: AgentResponse = {
      ...mockAgent,
      agent: {
        ...mockAgent.agent,
        repository: { url: "javascript:alert(1)" },
      },
    }
    render(<AgentCard agent={unsafeAgent} />)
    expect(screen.queryByRole("link", { name: "Repo" })).not.toBeInTheDocument()
    expect(screen.getByText("Repo")).toBeInTheDocument()
  })

  it("calls onClick when card is clicked", async () => {
    const onClick = vi.fn()
    render(<AgentCard agent={mockAgent} onClick={onClick} />)
    await userEvent.click(screen.getByText("test-agent"))
    expect(onClick).toHaveBeenCalledOnce()
  })

  it("renders without optional fields", () => {
    const minimalAgent: AgentResponse = {
      agent: {
        name: "minimal-agent",
        description: "",
        version: "0.1.0",
        framework: "custom",
        language: "go",
        image: "",
        modelProvider: "",
        modelName: "",
      },
      _meta: {},
    }
    render(<AgentCard agent={minimalAgent} />)
    expect(screen.getByText("minimal-agent")).toBeInTheDocument()
  })
})
