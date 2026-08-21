import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, it, expect, vi } from "vitest"
import { SkillCard } from "../skill-card"
import type { SkillResponse } from "@/lib/api/types.gen"

const mockSkill: SkillResponse = {
  skill: {
    name: "code-review",
    title: "Code Review",
    description: "Analyzes pull requests for quality and security.",
    version: "1.3.0",
    repository: {
      url: "https://github.com/example/code-review-skill",
      source: "github",
    },
    websiteUrl: "https://example.com/code-review",
    packages: [
      { identifier: "code-review-core", registryType: "npm", version: "1.3.0", transport: { type: "stdio" } },
      { identifier: "code-review-cli", registryType: "npm", version: "1.3.0", transport: { type: "stdio" } },
    ],
    remotes: [{ url: "https://remote.example.com/code-review" }],
  },
  _meta: {
    "io.modelcontextprotocol.registry/official": {
      publishedAt: "2025-03-10T00:00:00Z",
      updatedAt: "2025-09-01T00:00:00Z",
      status: "active",
      isLatest: true,
    },
  },
}

describe("SkillCard", () => {
  it("renders title as heading", () => {
    render(<SkillCard skill={mockSkill} />)
    expect(screen.getByText("Code Review")).toBeInTheDocument()
  })

  it("renders description and version", () => {
    render(<SkillCard skill={mockSkill} />)
    expect(screen.getByText("Analyzes pull requests for quality and security.")).toBeInTheDocument()
    expect(screen.getByText("1.3.0")).toBeInTheDocument()
  })

  it("renders package and remote counts", () => {
    render(<SkillCard skill={mockSkill} />)
    expect(screen.getByText("2")).toBeInTheDocument()
    const ones = screen.getAllByText("1")
    expect(ones.length).toBeGreaterThanOrEqual(1)
  })

  it("renders repository source", () => {
    render(<SkillCard skill={mockSkill} />)
    expect(screen.getByText("github")).toBeInTheDocument()
  })

  it("renders the ownership display name instead of the subject", () => {
    const ownedSkill: SkillResponse = {
      ...mockSkill,
      _meta: {
        ...mockSkill._meta,
        "aregistry.ai/ownership": {
          displayName: "Ada Lovelace",
          subject: "oidc-subject",
        },
      },
    }
    render(<SkillCard skill={ownedSkill} />)
    expect(screen.getByText("Ada Lovelace")).toBeInTheDocument()
    expect(screen.queryByText("oidc-subject")).not.toBeInTheDocument()
  })

  it("falls back to the ownership subject when the display name is empty", () => {
    const ownedSkill: SkillResponse = {
      ...mockSkill,
      _meta: {
        ...mockSkill._meta,
        "aregistry.ai/ownership": {
          displayName: "",
          subject: "github-user",
        },
      },
    }
    render(<SkillCard skill={ownedSkill} />)
    expect(screen.getByText("github-user")).toBeInTheDocument()
  })

  it("renders a placeholder when ownership is absent", () => {
    render(<SkillCard skill={mockSkill} />)
    expect(screen.getByText("Unknown")).toBeInTheDocument()
  })

  it("does not render a last-modified element when updatedAt is absent", () => {
    const skillWithoutUpdatedAt: SkillResponse = {
      ...mockSkill,
      _meta: {},
    }
    render(<SkillCard skill={skillWithoutUpdatedAt} />)
    expect(screen.queryByText("Last modified")).not.toBeInTheDocument()
  })

  it("renders HTML in a display name as literal text", () => {
    const ownedSkill: SkillResponse = {
      ...mockSkill,
      _meta: {
        ...mockSkill._meta,
        "aregistry.ai/ownership": {
          displayName: "<script>alert(1)</script>",
          subject: "oidc-subject",
        },
      },
    }
    const { container } = render(<SkillCard skill={ownedSkill} />)
    expect(screen.getByText("<script>alert(1)</script>")).toBeInTheDocument()
    expect(container.querySelector("script")).not.toBeInTheDocument()
  })

  it("does not render actions for javascript URLs", () => {
    const unsafeSkill: SkillResponse = {
      ...mockSkill,
      skill: {
        ...mockSkill.skill,
        repository: { url: "javascript:alert(1)", source: "github" },
        websiteUrl: "javascript:alert(1)",
      },
    }
    render(<SkillCard skill={unsafeSkill} />)
    expect(screen.queryByRole("button")).not.toBeInTheDocument()
  })

  it("falls back to name when title is not set", () => {
    const noTitle: SkillResponse = {
      skill: { ...mockSkill.skill, title: undefined },
      _meta: {},
    }
    render(<SkillCard skill={noTitle} />)
    expect(screen.getByText("code-review")).toBeInTheDocument()
  })

  it("calls onClick when card is clicked", async () => {
    const onClick = vi.fn()
    render(<SkillCard skill={mockSkill} onClick={onClick} />)
    await userEvent.click(screen.getByText("Code Review"))
    expect(onClick).toHaveBeenCalledOnce()
  })

  it("shows delete button when showDelete is true", () => {
    const onDelete = vi.fn()
    render(<SkillCard skill={mockSkill} showDelete onDelete={onDelete} />)
    const buttons = screen.getAllByRole("button")
    const deleteBtn = buttons.find(btn => btn.querySelector(".lucide-trash2"))
    expect(deleteBtn).toBeTruthy()
  })

  it("calls onDelete without triggering onClick", async () => {
    const onDelete = vi.fn()
    const onClick = vi.fn()
    render(<SkillCard skill={mockSkill} showDelete onDelete={onDelete} onClick={onClick} />)
    const buttons = screen.getAllByRole("button")
    const deleteBtn = buttons.find(btn => btn.querySelector(".lucide-trash2"))
    await userEvent.click(deleteBtn!)
    expect(onDelete).toHaveBeenCalledOnce()
    expect(onClick).not.toHaveBeenCalled()
  })
})
