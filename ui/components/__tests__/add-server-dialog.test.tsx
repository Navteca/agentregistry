import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { AddServerDialog } from "../add-server-dialog"
import { createServerV0, scoreServerV0 } from "@/lib/admin-api"
import { toast } from "sonner"

vi.mock("@/lib/admin-api", () => ({
  createServerV0: vi.fn(),
  scoreServerV0: vi.fn(),
}))

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

describe("AddServerDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(createServerV0).mockResolvedValue({
      data: { server: { name: "io.navteca/hello-mcp", version: "0.1.8" } },
    } as never)
    vi.mocked(scoreServerV0).mockResolvedValue({
      data: { scores: { total: 93 } },
    } as never)
  })

  it("defaults new packages to oci registry type", async () => {
    const user = userEvent.setup()

    render(<AddServerDialog open onOpenChange={() => { }} onServerAdded={() => { }} />)

    await user.click(screen.getByRole("button", { name: "Add Package" }))

    expect(screen.getByDisplayValue("oci")).toBeInTheDocument()
  })

  it("prevents submit when package transport is streamable-http without URL", async () => {
    const user = userEvent.setup()

    render(<AddServerDialog open onOpenChange={() => { }} onServerAdded={() => { }} />)

    await user.type(screen.getByLabelText("Server Name *"), "io.navteca/hello-mcp")
    await user.type(screen.getByLabelText("Version *"), "0.1.8")
    await user.type(screen.getByLabelText("Description *"), "MCP server built with FastMCP")
    await user.type(screen.getByLabelText("Repository URL *"), "https://github.com/navteca/hello-mcp")

    await user.click(screen.getByRole("button", { name: "Add Package" }))
    await user.type(screen.getByPlaceholderText("Package identifier"), "docker.io/luisgleon/my-mcp-server:0.1.8")
    await user.type(screen.getByPlaceholderText("Version"), "0.1.8")
    await user.click(screen.getByRole("radio", { name: "streamable-http" }))

    expect(
      screen.getByPlaceholderText("Transport URL (required) e.g. http://localhost:8080/mcp"),
    ).toBeInTheDocument()

    await user.click(screen.getByRole("button", { name: "Create Server" }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("Package transport URL is required for streamable-http")
    })
    expect(createServerV0).not.toHaveBeenCalled()
  })

  it("sends transport URL in package payload for streamable-http", async () => {
    const user = userEvent.setup()

    render(<AddServerDialog open onOpenChange={() => { }} onServerAdded={() => { }} />)

    await user.type(screen.getByLabelText("Server Name *"), "io.navteca/hello-mcp")
    await user.type(screen.getByLabelText("Version *"), "0.1.8")
    await user.type(screen.getByLabelText("Description *"), "MCP server built with FastMCP")
    await user.type(screen.getByLabelText("Repository URL *"), "https://github.com/navteca/hello-mcp")

    await user.click(screen.getByRole("button", { name: "Add Package" }))
    await user.type(screen.getByPlaceholderText("Package identifier"), "docker.io/luisgleon/my-mcp-server:0.1.8")
    await user.type(screen.getByPlaceholderText("Version"), "0.1.8")
    await user.click(screen.getByRole("radio", { name: "streamable-http" }))
    await user.type(
      screen.getByPlaceholderText("Transport URL (required) e.g. http://localhost:8080/mcp"),
      "http://localhost:8080/mcp",
    )

    await user.click(screen.getByRole("button", { name: "Create Server" }))

    await waitFor(() => {
      expect(createServerV0).toHaveBeenCalledTimes(1)
    })

    const callArg = vi.mocked(createServerV0).mock.calls[0]?.[0]
    expect(callArg?.throwOnError).toBe(true)
    expect(callArg?.body.$schema).toBe("https://static.modelcontextprotocol.io/schemas/2025-10-17/server.schema.json")
    expect(callArg?.body.packages).toEqual([
      {
        identifier: "docker.io/luisgleon/my-mcp-server:0.1.8",
        version: "0.1.8",
        registryType: "oci",
        transport: {
          type: "streamable-http",
          url: "http://localhost:8080/mcp",
        },
      },
    ])
  })

  it("prevents submit when remote transport requires URL and it is empty", async () => {
    const user = userEvent.setup()

    render(<AddServerDialog open onOpenChange={() => { }} onServerAdded={() => { }} />)

    await user.type(screen.getByLabelText("Server Name *"), "io.navteca/hello-mcp")
    await user.type(screen.getByLabelText("Version *"), "0.1.8")
    await user.type(screen.getByLabelText("Description *"), "MCP server built with FastMCP")
    await user.type(screen.getByLabelText("Repository URL *"), "https://github.com/navteca/hello-mcp")

    await user.click(screen.getByRole("button", { name: "Add Remote" }))
    await user.click(screen.getByRole("button", { name: "Create Server" }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("Remote URL is required for sse")
    })
    expect(createServerV0).not.toHaveBeenCalled()
  })

  it("requires repository URL before creating a server", async () => {
    const user = userEvent.setup()

    render(<AddServerDialog open onOpenChange={() => { }} onServerAdded={() => { }} />)

    await user.type(screen.getByLabelText("Server Name *"), "io.navteca/hello-mcp")
    await user.type(screen.getByLabelText("Version *"), "0.1.8")
    await user.type(screen.getByLabelText("Description *"), "MCP server built with FastMCP")

    expect(screen.getByRole("button", { name: "Create Server" })).toBeDisabled()
    expect(createServerV0).not.toHaveBeenCalled()
  })

  it("prevents submit when repository URL does not match selected provider", async () => {
    const user = userEvent.setup()

    render(<AddServerDialog open onOpenChange={() => { }} onServerAdded={() => { }} />)

    await user.type(screen.getByLabelText("Server Name *"), "io.navteca/hello-mcp")
    await user.type(screen.getByLabelText("Version *"), "0.1.8")
    await user.type(screen.getByLabelText("Description *"), "MCP server built with FastMCP")
    await user.type(screen.getByLabelText("Repository URL *"), "https://gitlab.com/navteca/hello-mcp")

    await user.click(screen.getByRole("button", { name: "Create Server" }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("Repository URL must match the selected provider (github.com)")
    })
    expect(createServerV0).not.toHaveBeenCalled()
  })

  it("sends repository when URL matches selected provider", async () => {
    const user = userEvent.setup()

    render(<AddServerDialog open onOpenChange={() => { }} onServerAdded={() => { }} />)

    await user.type(screen.getByLabelText("Server Name *"), "io.navteca/hello-mcp")
    await user.type(screen.getByLabelText("Version *"), "0.1.8")
    await user.type(screen.getByLabelText("Description *"), "MCP server built with FastMCP")
    await user.click(screen.getByRole("combobox"))
    await user.click(screen.getByRole("option", { name: "GitLab" }))
    await user.type(screen.getByLabelText("Repository URL *"), "https://gitlab.com/navteca/hello-mcp")

    await user.click(screen.getByRole("button", { name: "Create Server" }))

    await waitFor(() => {
      expect(createServerV0).toHaveBeenCalledTimes(1)
    })

    const callArg = vi.mocked(createServerV0).mock.calls[0]?.[0]
    expect(callArg?.body.repository).toEqual({
      source: "gitlab",
      url: "https://gitlab.com/navteca/hello-mcp",
    })
  })

  it("triggers scoring after successful server creation", async () => {
    const user = userEvent.setup()

    render(<AddServerDialog open onOpenChange={() => { }} onServerAdded={() => { }} />)

    await user.type(screen.getByLabelText("Server Name *"), "io.navteca/hello-mcp")
    await user.type(screen.getByLabelText("Version *"), "0.1.8")
    await user.type(screen.getByLabelText("Description *"), "MCP server built with FastMCP")
    await user.type(screen.getByLabelText("Repository URL *"), "https://github.com/navteca/hello-mcp")

    await user.click(screen.getByRole("button", { name: "Create Server" }))

    await waitFor(() => {
      expect(createServerV0).toHaveBeenCalledTimes(1)
    })

    await waitFor(() => {
      expect(scoreServerV0).toHaveBeenCalledTimes(1)
    })

    const scoreCallArg = vi.mocked(scoreServerV0).mock.calls[0]?.[0]
    expect(scoreCallArg?.path.serverName).toBe(encodeURIComponent("io.navteca/hello-mcp"))
    expect(scoreCallArg?.path.version).toBe("0.1.8")
  })

  it("shows success toast when scoring succeeds", async () => {
    const user = userEvent.setup()

    render(<AddServerDialog open onOpenChange={() => { }} onServerAdded={() => { }} />)

    await user.type(screen.getByLabelText("Server Name *"), "io.navteca/hello-mcp")
    await user.type(screen.getByLabelText("Version *"), "0.1.8")
    await user.type(screen.getByLabelText("Description *"), "MCP server built with FastMCP")
    await user.type(screen.getByLabelText("Repository URL *"), "https://github.com/navteca/hello-mcp")

    await user.click(screen.getByRole("button", { name: "Create Server" }))

    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith("MCP scoring completed")
    })
  })

  it("shows error toast when scoring fails", async () => {
    vi.mocked(scoreServerV0).mockRejectedValueOnce(new Error("Service unavailable") as never)
    const user = userEvent.setup()

    render(<AddServerDialog open onOpenChange={() => { }} onServerAdded={() => { }} />)

    await user.type(screen.getByLabelText("Server Name *"), "io.navteca/hello-mcp")
    await user.type(screen.getByLabelText("Version *"), "0.1.8")
    await user.type(screen.getByLabelText("Description *"), "MCP server built with FastMCP")
    await user.type(screen.getByLabelText("Repository URL *"), "https://github.com/navteca/hello-mcp")

    await user.click(screen.getByRole("button", { name: "Create Server" }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("MCP scoring failed — you can retry from the Score tab")
    })
  })

  it("does not call scoring when server creation fails", async () => {
    vi.mocked(createServerV0).mockRejectedValueOnce(new Error("Server creation failed") as never)
    const user = userEvent.setup()

    render(<AddServerDialog open onOpenChange={() => { }} onServerAdded={() => { }} />)

    await user.type(screen.getByLabelText("Server Name *"), "io.navteca/hello-mcp")
    await user.type(screen.getByLabelText("Version *"), "0.1.8")
    await user.type(screen.getByLabelText("Description *"), "MCP server built with FastMCP")
    await user.type(screen.getByLabelText("Repository URL *"), "https://github.com/navteca/hello-mcp")

    await user.click(screen.getByRole("button", { name: "Create Server" }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalled()
    })
    expect(scoreServerV0).not.toHaveBeenCalled()
  })

  it("shows backend detail error when repository URL is unreachable", async () => {
    const user = userEvent.setup()
    vi.mocked(createServerV0).mockRejectedValueOnce({
      detail: "Failed to create server",
      errors: [
        {
          message: "repository URL is not reachable: https://github.com/navteca/missing-repo returned status 404",
        },
      ],
      status: 400,
      title: "Bad Request",
    } as never)

    render(<AddServerDialog open onOpenChange={() => { }} onServerAdded={() => { }} />)

    await user.type(screen.getByLabelText("Server Name *"), "io.navteca/hello-mcp")
    await user.type(screen.getByLabelText("Version *"), "0.1.8")
    await user.type(screen.getByLabelText("Description *"), "MCP server built with FastMCP")
    await user.type(screen.getByLabelText("Repository URL *"), "https://github.com/navteca/missing-repo")

    await user.click(screen.getByRole("button", { name: "Create Server" }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(
        "repository URL is not reachable: https://github.com/navteca/missing-repo returned status 404",
      )
    })
  })
})
