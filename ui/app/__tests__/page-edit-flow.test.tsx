import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, it, expect, vi, beforeEach } from "vitest"

import AdminPage from "../page"
import type { ServerResponse } from "@/lib/admin-api"
import { listServersV0, listSkillsV0, listAgentsV0, listPromptsV0, deleteServerVersionV0 } from "@/lib/admin-api"
import { capabilityFlags } from "@/lib/capabilities"

vi.mock("@/components/server-card", () => ({
  ServerCard: ({
    server,
    onEdit,
    showEdit,
    showDelete,
    showDeploy,
  }: {
    server: ServerResponse
    onEdit?: (server: ServerResponse) => void
    showEdit?: boolean
    showDelete?: boolean
    showDeploy?: boolean
  }) => (
    <div>
      <div>{server.server.name}</div>
      {showEdit && <button onClick={() => onEdit?.(server)}>Edit server {server.server.name}</button>}
      {showDelete && <button>Remove server {server.server.name}</button>}
      {showDeploy && <button>Deploy server {server.server.name}</button>}
    </div>
  ),
}))

vi.mock("@/components/edit-server-dialog", () => ({
  EditServerDialog: ({
    open,
    server,
    onOpenChange,
    onServerUpdated,
  }: {
    open: boolean
    server: ServerResponse | null
    onOpenChange: (open: boolean) => void
    onServerUpdated: () => void
  }) =>
    open ? (
      <div>
        <p>Editing {server?.server.name}</p>
        <button
          onClick={() => {
            onServerUpdated()
            onOpenChange(false)
          }}
        >
          Save edit
        </button>
      </div>
    ) : null,
}))

vi.mock("@/components/skill-card", () => ({ SkillCard: () => null }))
vi.mock("@/components/agent-card", () => ({ AgentCard: () => null }))
vi.mock("@/components/prompt-card", () => ({ PromptCard: () => null }))
vi.mock("@/components/server-detail", () => ({ ServerDetail: () => null }))
vi.mock("@/components/skill-detail", () => ({ SkillDetail: () => null }))
vi.mock("@/components/agent-detail", () => ({ AgentDetail: () => null }))
vi.mock("@/components/prompt-detail", () => ({ PromptDetail: () => null }))
vi.mock("@/components/import-dialog", () => ({ ImportDialog: () => null }))
vi.mock("@/components/add-server-dialog", () => ({ AddServerDialog: () => null }))
vi.mock("@/components/add-skill-dialog", () => ({ AddSkillDialog: () => null }))
vi.mock("@/components/add-agent-dialog", () => ({ AddAgentDialog: () => null }))
vi.mock("@/components/add-prompt-dialog", () => ({ AddPromptDialog: () => null }))
vi.mock("@/components/deploy-dialog", () => ({ DeployDialog: () => null }))
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

vi.mock("@/lib/admin-api", async () => {
  const actual = await vi.importActual("@/lib/admin-api")
  return {
    ...actual,
    listServersV0: vi.fn(),
    listSkillsV0: vi.fn(),
    listAgentsV0: vi.fn(),
    listPromptsV0: vi.fn(),
    deleteServerVersionV0: vi.fn(),
  }
})

const mockServer: ServerResponse = {
  server: {
    $schema: "https://static.modelcontextprotocol.io/schemas/2025-10-17/server.schema.json",
    name: "io.navteca/hello-mcp",
    description: "hello server",
    version: "0.1.8",
  },
  _meta: {
    "aregistry.ai/capabilities": {
      can_update: true,
      can_delete: false,
      can_deploy: false,
    },
  },
}

describe("AdminPage edit server flow", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(listServersV0).mockResolvedValue({
      data: { servers: [mockServer], metadata: { count: 1, nextCursor: undefined } },
    } as never)
    vi.mocked(listSkillsV0).mockResolvedValue({
      data: { skills: [], metadata: { count: 0, nextCursor: undefined } },
    } as never)
    vi.mocked(listAgentsV0).mockResolvedValue({
      data: { agents: [], metadata: { count: 0, nextCursor: undefined } },
    } as never)
    vi.mocked(listPromptsV0).mockResolvedValue({
      data: { prompts: [], metadata: { count: 0, nextCursor: undefined } },
    } as never)
    vi.mocked(deleteServerVersionV0).mockResolvedValue({ data: { message: "ok" } } as never)
  })

  it("opens edit dialog from server row and refreshes after save", async () => {
    const user = userEvent.setup()
    render(<AdminPage />)

    await screen.findByText("io.navteca/hello-mcp")
    await user.click(screen.getByRole("button", { name: "Edit server io.navteca/hello-mcp" }))

    expect(screen.getByText("Editing io.navteca/hello-mcp")).toBeInTheDocument()

    await user.click(screen.getByRole("button", { name: "Save edit" }))

    await waitFor(() => {
      expect(screen.queryByText("Editing io.navteca/hello-mcp")).not.toBeInTheDocument()
      expect(listServersV0).toHaveBeenCalledTimes(2)
    })
  })

  it("passes all server capability flags to the card controls", async () => {
    const serverWithCapabilities: ServerResponse = {
      ...mockServer,
      _meta: {
        "aregistry.ai/capabilities": {
          can_update: true,
          can_delete: true,
          can_deploy: true,
        },
      },
    }
    vi.mocked(listServersV0).mockResolvedValue({
      data: { servers: [serverWithCapabilities], metadata: { count: 1, nextCursor: undefined } },
    } as never)

    render(<AdminPage />)

    await screen.findByText("io.navteca/hello-mcp")
    const flags = capabilityFlags(serverWithCapabilities._meta["aregistry.ai/capabilities"])
    expect(screen.queryByRole("button", { name: "Edit server io.navteca/hello-mcp" })).toBeInTheDocument()
    expect(screen.queryByRole("button", { name: "Remove server io.navteca/hello-mcp" })).toBeInTheDocument()
    expect(screen.queryByRole("button", { name: "Deploy server io.navteca/hello-mcp" })).toBeInTheDocument()
    expect(flags).toEqual({ showEdit: true, showDelete: true, showDeploy: true, showReview: false, showOverride: false })
  })

  it("hides all server controls when capabilities are absent", async () => {
    vi.mocked(listServersV0).mockResolvedValue({
      data: { servers: [{ ...mockServer, _meta: {} }], metadata: { count: 1, nextCursor: undefined } },
    } as never)

    render(<AdminPage />)

    await screen.findByText("io.navteca/hello-mcp")
    expect(screen.queryByRole("button", { name: /Edit server/ })).not.toBeInTheDocument()
    expect(screen.queryByRole("button", { name: /Remove server/ })).not.toBeInTheDocument()
    expect(screen.queryByRole("button", { name: /Deploy server/ })).not.toBeInTheDocument()
  })
})
