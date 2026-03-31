import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { EditServerDialog } from "../edit-server-dialog"
import { editServerV0, type ServerResponse } from "@/lib/admin-api"
import { toast } from "sonner"

vi.mock("@/lib/admin-api", () => ({
  editServerV0: vi.fn(),
}))

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

const mockServer: ServerResponse = {
  server: {
    $schema: "https://static.modelcontextprotocol.io/schemas/2025-10-17/server.schema.json",
    name: "io.navteca/hello-mcp",
    title: "Hello MCP",
    description: "Initial description",
    version: "0.1.8",
    websiteUrl: "https://navteca.io/hello-mcp",
    repository: {
      source: "github",
      url: "https://github.com/navteca/hello-mcp",
    },
    packages: [
      {
        identifier: "ghcr.io/navteca/hello-mcp",
        version: "0.1.8",
        registryType: "oci",
        transport: { type: "stdio" },
      },
    ],
    remotes: [
      {
        type: "sse",
        url: "https://api.navteca.io/mcp/sse",
      },
    ],
    _meta: {
      "io.modelcontextprotocol.registry/publisher-provided": {
        "aregistry.ai/metadata": {
          stars: 42,
        },
      },
    },
  },
  _meta: {},
}

describe("EditServerDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(editServerV0).mockResolvedValue({ data: mockServer } as never)
  })

  it("prefills form fields from selected server and keeps name/version immutable", () => {
    render(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={mockServer}
        onServerUpdated={() => { }}
      />,
    )

    expect(screen.getByLabelText("Server Name *")).toHaveValue("io.navteca/hello-mcp")
    expect(screen.getByLabelText("Version *")).toHaveValue("0.1.8")
    expect(screen.getByLabelText("Description *")).toHaveValue("Initial description")
    expect(screen.getByLabelText("Server Name *")).toBeDisabled()
    expect(screen.getByLabelText("Version *")).toBeDisabled()
  })

  it("renders empty package/remote states when source data is missing", () => {
    const sparseServer: ServerResponse = {
      server: {
        ...mockServer.server,
        packages: undefined,
        remotes: undefined,
        repository: {
          source: "bitbucket",
          url: "https://bitbucket.org/navteca/hello-mcp",
        },
      },
      _meta: {},
    }
    render(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={sparseServer}
        onServerUpdated={() => { }}
      />,
    )

    expect(screen.getByText("No packages added")).toBeInTheDocument()
    expect(screen.getByText("No remotes added")).toBeInTheDocument()
  })

  it("falls back to empty version and repository url on prefill", () => {
    const fallbackServer = {
      server: {
        ...mockServer.server,
        version: "",
        repository: undefined,
      },
      _meta: {},
    } as unknown as ServerResponse

    render(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={fallbackServer}
        onServerUpdated={() => { }}
      />,
    )

    expect(screen.getByLabelText("Version *")).toHaveValue("")
    expect(screen.getByLabelText("Repository URL")).toHaveValue("")
  })

  it("falls back optional prefill fields when source values are missing", () => {
    const fallbackServer = {
      server: {
        ...mockServer.server,
        $schema: "",
        name: "",
        description: "",
        remotes: [{ type: "stdio" }],
      },
      _meta: {},
    } as unknown as ServerResponse

    render(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={fallbackServer}
        onServerUpdated={() => { }}
      />,
    )

    expect(screen.getByLabelText("Server Name *")).toHaveValue("")
    expect(screen.getByLabelText("Description *")).toHaveValue("")
    expect(screen.getByPlaceholderText("URL (optional)")).toHaveValue("")
  })

  it("does not render dialog content when closed", () => {
    render(
      <EditServerDialog
        open={false}
        onOpenChange={() => { }}
        server={mockServer}
        onServerUpdated={() => { }}
      />,
    )

    expect(screen.queryByText("Edit MCP Server")).not.toBeInTheDocument()
  })

  it("closes on cancel without API call", async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()
    render(
      <EditServerDialog
        open
        onOpenChange={onOpenChange}
        server={mockServer}
        onServerUpdated={() => { }}
      />,
    )

    await user.click(screen.getByRole("button", { name: "Cancel" }))

    expect(onOpenChange).toHaveBeenCalledWith(false)
    expect(editServerV0).not.toHaveBeenCalled()
  })

  it("submits updates with correct path and payload then refreshes", async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()
    const onServerUpdated = vi.fn()

    render(
      <EditServerDialog
        open
        onOpenChange={onOpenChange}
        server={mockServer}
        onServerUpdated={onServerUpdated}
      />,
    )

    await user.clear(screen.getByLabelText("Description *"))
    await user.type(screen.getByLabelText("Description *"), "Updated description")
    await user.clear(screen.getByLabelText("Display Title"))
    await user.type(screen.getByLabelText("Display Title"), "Updated MCP")
    await user.clear(screen.getByLabelText("Website URL"))
    await user.type(screen.getByLabelText("Website URL"), "https://navteca.io/updated")
    await user.click(screen.getByRole("button", { name: "Save Changes" }))

    await waitFor(() => {
      expect(editServerV0).toHaveBeenCalledTimes(1)
    })

    const callArg = vi.mocked(editServerV0).mock.calls[0]?.[0]
    expect(callArg?.path).toEqual({
      serverName: "io.navteca/hello-mcp",
      version: "0.1.8",
    })
    expect(callArg?.body.description).toBe("Updated description")
    expect(callArg?.body.title).toBe("Updated MCP")
    expect(callArg?.body.websiteUrl).toBe("https://navteca.io/updated")
    expect(callArg?.body.name).toBe("io.navteca/hello-mcp")
    expect(callArg?.body.version).toBe("0.1.8")
    expect(callArg?.throwOnError).toBe(true)

    await waitFor(() => {
      expect(onOpenChange).toHaveBeenCalledWith(false)
      expect(onServerUpdated).toHaveBeenCalledOnce()
      expect(toast.success).toHaveBeenCalledWith('Server "io.navteca/hello-mcp" updated successfully!')
    })
  })

  it("shows validation error for invalid repository host", async () => {
    const user = userEvent.setup()
    render(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={mockServer}
        onServerUpdated={() => { }}
      />,
    )

    await user.clear(screen.getByLabelText("Repository URL"))
    await user.type(screen.getByLabelText("Repository URL"), "https://gitlab.com/navteca/hello-mcp")
    await user.click(screen.getByRole("button", { name: "Save Changes" }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("Repository URL must match the selected provider (github.com)")
    })
    expect(editServerV0).not.toHaveBeenCalled()
  })

  it("shows validation error for malformed repository URL", async () => {
    const user = userEvent.setup()
    render(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={mockServer}
        onServerUpdated={() => { }}
      />,
    )

    await user.clear(screen.getByLabelText("Repository URL"))
    await user.type(screen.getByLabelText("Repository URL"), "not-a-valid-url")
    await user.click(screen.getByRole("button", { name: "Save Changes" }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("Repository URL must be a valid absolute URL")
    })
  })

  it("shows validation error for invalid repository protocol", async () => {
    const user = userEvent.setup()
    render(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={mockServer}
        onServerUpdated={() => { }}
      />,
    )

    await user.clear(screen.getByLabelText("Repository URL"))
    await user.type(screen.getByLabelText("Repository URL"), "ftp://github.com/navteca/hello-mcp")
    await user.click(screen.getByRole("button", { name: "Save Changes" }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("Repository URL must use http or https")
    })
    expect(editServerV0).not.toHaveBeenCalled()
  })

  it("shows 400 error detail and keeps dialog open", async () => {
    const user = userEvent.setup()
    vi.mocked(editServerV0).mockRejectedValueOnce({
      status: 400,
      errors: [{ message: "description cannot be empty" }],
    } as never)
    const onOpenChange = vi.fn()

    render(
      <EditServerDialog
        open
        onOpenChange={onOpenChange}
        server={mockServer}
        onServerUpdated={() => { }}
      />,
    )

    await user.clear(screen.getByLabelText("Description *"))
    await user.type(screen.getByLabelText("Description *"), "Still valid")
    await user.click(screen.getByRole("button", { name: "Save Changes" }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("description cannot be empty")
    })
    expect(onOpenChange).not.toHaveBeenCalledWith(false)
  })

  it("shows permission error for unauthorized responses", async () => {
    const user = userEvent.setup()
    vi.mocked(editServerV0).mockRejectedValueOnce({ detail: "forbidden" } as never)

    render(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={mockServer}
        onServerUpdated={() => { }}
      />,
    )

    await user.click(screen.getByRole("button", { name: "Save Changes" }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("forbidden")
    })
  })

  it("surfaces string errors directly", async () => {
    const user = userEvent.setup()
    vi.mocked(editServerV0).mockRejectedValueOnce("raw string failure" as never)
    render(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={mockServer}
        onServerUpdated={() => { }}
      />,
    )

    await user.click(screen.getByRole("button", { name: "Save Changes" }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("raw string failure")
    })
  })

  it("surfaces nested error details", async () => {
    const user = userEvent.setup()
    vi.mocked(editServerV0).mockRejectedValueOnce({
      error: {
        detail: "nested detail failure",
      },
    } as never)
    render(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={mockServer}
        onServerUpdated={() => { }}
      />,
    )

    await user.click(screen.getByRole("button", { name: "Save Changes" }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("nested detail failure")
    })
  })

  it("surfaces top-level message errors", async () => {
    const user = userEvent.setup()
    vi.mocked(editServerV0).mockRejectedValueOnce({ message: "top-level message" } as never)
    render(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={mockServer}
        onServerUpdated={() => { }}
      />,
    )

    await user.click(screen.getByRole("button", { name: "Save Changes" }))
    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("top-level message")
    })
  })

  it("surfaces title fallback when errors array has no message", async () => {
    const user = userEvent.setup()
    vi.mocked(editServerV0).mockRejectedValueOnce({ errors: [{}], title: "title fallback" } as never)
    render(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={mockServer}
        onServerUpdated={() => { }}
      />,
    )

    await user.click(screen.getByRole("button", { name: "Save Changes" }))
    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("title fallback")
    })
  })

  it("falls back when errors array contains non-object entries", async () => {
    const user = userEvent.setup()
    vi.mocked(editServerV0).mockRejectedValueOnce({ errors: [null], title: "non-object error item" } as never)
    render(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={mockServer}
        onServerUpdated={() => { }}
      />,
    )

    await user.click(screen.getByRole("button", { name: "Save Changes" }))
    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("non-object error item")
    })
  })

  it("falls back to generic message for non-object/non-string errors", async () => {
    const user = userEvent.setup()
    vi.mocked(editServerV0).mockRejectedValueOnce(42 as never)
    render(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={mockServer}
        onServerUpdated={() => { }}
      />,
    )

    await user.click(screen.getByRole("button", { name: "Save Changes" }))
    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("Failed to update server")
    })
  })

  it("shows generic fallback error for unknown failures", async () => {
    const user = userEvent.setup()
    vi.mocked(editServerV0).mockRejectedValueOnce({} as never)

    render(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={mockServer}
        onServerUpdated={() => { }}
      />,
    )

    await user.click(screen.getByRole("button", { name: "Save Changes" }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("Failed to update server")
    })
  })

  it("validates transport URL when package transport requires it", async () => {
    const user = userEvent.setup()
    render(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={mockServer}
        onServerUpdated={() => { }}
      />,
    )

    await user.click(screen.getByRole("button", { name: "Add Package" }))
    const packageIdentifiers = screen.getAllByPlaceholderText("Package identifier")
    const packageVersions = screen.getAllByPlaceholderText("Version")
    await user.type(packageIdentifiers[1], "docker.io/navteca/new")
    await user.type(packageVersions[1], "1.0.0")
    await user.click(screen.getAllByRole("radio", { name: "streamable-http" })[1])
    await user.click(screen.getByRole("button", { name: "Save Changes" }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("Package transport URL is required for streamable-http")
    })
    expect(editServerV0).not.toHaveBeenCalled()
  })

  it("submits package transport URL when required transport is selected", async () => {
    const user = userEvent.setup()
    render(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={mockServer}
        onServerUpdated={() => { }}
      />,
    )

    await user.click(screen.getByRole("button", { name: "Add Package" }))
    const packageIdentifiers = screen.getAllByPlaceholderText("Package identifier")
    const packageVersions = screen.getAllByPlaceholderText("Version")
    await user.type(packageIdentifiers[1], "docker.io/navteca/new")
    await user.type(packageVersions[1], "1.0.0")
    await user.click(screen.getAllByRole("radio", { name: "streamable-http" })[1])
    await user.type(
      screen.getByPlaceholderText("Transport URL (required) e.g. http://localhost:8080/mcp"),
      "http://localhost:8080/mcp",
    )

    await user.click(screen.getByRole("button", { name: "Save Changes" }))

    await waitFor(() => {
      expect(editServerV0).toHaveBeenCalledTimes(1)
    })
    const callArg = vi.mocked(editServerV0).mock.calls[0]?.[0]
    expect(callArg?.body.packages).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          identifier: "docker.io/navteca/new",
          transport: { type: "streamable-http", url: "http://localhost:8080/mcp" },
        }),
      ]),
    )
  })

  it("validates remote URL when remote type requires it", async () => {
    const user = userEvent.setup()
    render(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={mockServer}
        onServerUpdated={() => { }}
      />,
    )

    await user.click(screen.getByRole("button", { name: "Add Remote" }))
    await user.click(screen.getByRole("button", { name: "Save Changes" }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("Remote URL is required for sse")
    })
    expect(editServerV0).not.toHaveBeenCalled()
  })

  it("updates and removes remotes via remote controls", async () => {
    const user = userEvent.setup()
    const { container } = render(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={mockServer}
        onServerUpdated={() => { }}
      />,
    )

    await user.click(screen.getByRole("button", { name: "Add Remote" }))
    const remoteTypeSelects = container.querySelectorAll("select")
    await user.selectOptions(remoteTypeSelects[remoteTypeSelects.length - 1], "stdio")
    const remoteUrlFields = screen.getAllByPlaceholderText(/URL/)
    await user.type(remoteUrlFields[1], "http://localhost:7777")

    await user.click(screen.getByRole("button", { name: "Remove remote 2" }))
    await user.click(screen.getByRole("button", { name: "Save Changes" }))

    await waitFor(() => {
      expect(editServerV0).toHaveBeenCalledTimes(1)
    })
  })

  it("updates repository source, package registry type, and removes package", async () => {
    const user = userEvent.setup()
    const { container } = render(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={mockServer}
        onServerUpdated={() => { }}
      />,
    )

    await user.click(screen.getAllByRole("combobox")[0])
    await user.click(screen.getByRole("option", { name: "GitLab" }))
    await user.clear(screen.getByLabelText("Repository URL"))
    await user.type(screen.getByLabelText("Repository URL"), "https://gitlab.com/navteca/hello-mcp")

    const packageRegistrySelects = container.querySelectorAll("select")
    await user.selectOptions(packageRegistrySelects[0], "npm")
    await user.click(screen.getByRole("button", { name: "Remove package 1" }))
    await user.click(screen.getByRole("button", { name: "Save Changes" }))

    await waitFor(() => {
      expect(editServerV0).toHaveBeenCalledTimes(1)
    })

    const callArg = vi.mocked(editServerV0).mock.calls[0]?.[0]
    expect(callArg?.body.repository).toEqual({
      source: "gitlab",
      url: "https://gitlab.com/navteca/hello-mcp",
    })
    expect(callArg?.body.packages).toBeUndefined()
  })

  it("allows clearing repository URL and omits repository on submit", async () => {
    const user = userEvent.setup()
    render(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={mockServer}
        onServerUpdated={() => { }}
      />,
    )

    await user.clear(screen.getByLabelText("Repository URL"))
    await user.click(screen.getByRole("button", { name: "Save Changes" }))

    await waitFor(() => {
      expect(editServerV0).toHaveBeenCalledTimes(1)
    })

    const callArg = vi.mocked(editServerV0).mock.calls[0]?.[0]
    expect(callArg?.body.repository).toBeUndefined()
  })

  it("normalizes empty website, package transport fallback, and stdio remote url", async () => {
    const user = userEvent.setup()
    const oddServer: ServerResponse = {
      server: {
        ...mockServer.server,
        websiteUrl: "",
        packages: [
          {
            identifier: "ghcr.io/navteca/odd",
            version: "1.0.0",
            registryType: "oci",
            transport: { type: "" },
          },
        ],
        remotes: [
          {
            type: "stdio",
            url: "",
          },
        ],
      },
      _meta: {},
    }

    render(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={oddServer}
        onServerUpdated={() => { }}
      />,
    )

    await user.clear(screen.getByLabelText("Website URL"))
    await user.click(screen.getByRole("button", { name: "Save Changes" }))

    await waitFor(() => {
      expect(editServerV0).toHaveBeenCalledTimes(1)
    })

    const callArg = vi.mocked(editServerV0).mock.calls[0]?.[0]
    expect(callArg?.body.websiteUrl).toBeUndefined()
    expect(callArg?.body.packages?.[0]?.transport).toEqual({ type: "stdio" })
    expect(callArg?.body.remotes?.[0]).toEqual({ type: "stdio", url: undefined })
  })

  it("prefills package and remote defaults when source entries are partial", () => {
    const fallbackServer = {
      server: {
        ...mockServer.server,
        packages: [{}],
        remotes: [{}],
      },
      _meta: {},
    } as unknown as ServerResponse

    render(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={fallbackServer}
        onServerUpdated={() => { }}
      />,
    )

    expect(screen.getByPlaceholderText("Package identifier")).toHaveValue("")
    expect(screen.getByDisplayValue("oci")).toBeInTheDocument()
    expect(screen.getByDisplayValue("sse")).toBeInTheDocument()
  })

  it("submits undefined optional fields when title, repository, and remotes are absent", async () => {
    const user = userEvent.setup()
    const minimalServer: ServerResponse = {
      server: {
        ...mockServer.server,
        title: "",
        repository: undefined,
        remotes: undefined,
        packages: undefined,
      },
      _meta: {},
    }

    render(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={minimalServer}
        onServerUpdated={() => { }}
      />,
    )

    await user.click(screen.getByRole("button", { name: "Save Changes" }))
    await waitFor(() => {
      expect(editServerV0).toHaveBeenCalledTimes(1)
    })

    const callArg = vi.mocked(editServerV0).mock.calls[0]?.[0]
    expect(callArg?.body.title).toBeUndefined()
    expect(callArg?.body.repository).toBeUndefined()
    expect(callArg?.body.remotes).toBeUndefined()
  })

  it("no-ops submit when server becomes null while dialog remains open", async () => {
    const user = userEvent.setup()
    const { rerender } = render(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={mockServer}
        onServerUpdated={() => { }}
      />,
    )

    rerender(
      <EditServerDialog
        open
        onOpenChange={() => { }}
        server={null}
        onServerUpdated={() => { }}
      />,
    )

    await user.click(screen.getByRole("button", { name: "Save Changes" }))
    expect(editServerV0).not.toHaveBeenCalled()
  })
})
