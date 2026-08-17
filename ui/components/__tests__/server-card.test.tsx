import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, it, expect, vi } from "vitest"
import { ServerCard } from "../server-card"
import type { CapabilitiesMeta, ServerResponse } from "@/lib/api/types.gen"
import { capabilityFlags } from "@/lib/capabilities"

const mockServer: ServerResponse = {
  server: {
    $schema: "https://modelcontextprotocol.io/schemas/server.json",
    name: "acme/database-server",
    title: "Database Server",
    description: "MCP server for PostgreSQL with connection pooling.",
    version: "3.2.1",
    repository: {
      url: "https://github.com/acme/database-server",
      source: "github",
    },
    websiteUrl: "https://acme.dev/database-server",
    packages: [
      {
        registryType: "npm",
        identifier: "@acme/database-server",
        transport: { type: "stdio" },
      },
    ],
    remotes: [
      {
        type: "streamable-http",
        url: "https://mcp.acme.dev/database",
      },
    ],
  },
  _meta: {
    "io.modelcontextprotocol.registry/official": {
      publishedAt: "2024-11-01T00:00:00Z",
      updatedAt: "2025-08-20T00:00:00Z",
      status: "active",
      isLatest: true,
    },
  },
}

function serverWithCapabilities(capabilities?: CapabilitiesMeta, packages = mockServer.server.packages): ServerResponse {
  return {
    ...mockServer,
    server: { ...mockServer.server, packages },
    _meta: {
      ...mockServer._meta,
      ...(capabilities ? { "aregistry.ai/capabilities": capabilities } : {}),
    },
  }
}

function renderWithCapabilities(server: ServerResponse) {
  const flags = capabilityFlags(server._meta["aregistry.ai/capabilities"])
  return render(
    <ServerCard
      server={server}
      {...flags}
      onEdit={vi.fn()}
      onDelete={vi.fn()}
      onDeploy={vi.fn()}
    />,
  )
}

describe("ServerCard", () => {
  it("renders title as heading", () => {
    render(<ServerCard server={mockServer} />)
    expect(screen.getByText("Database Server")).toBeInTheDocument()
  })

  it("renders description and version", () => {
    render(<ServerCard server={mockServer} />)
    expect(screen.getByText("MCP server for PostgreSQL with connection pooling.")).toBeInTheDocument()
    expect(screen.getByText("3.2.1")).toBeInTheDocument()
  })

  it("renders package and remote counts", () => {
    render(<ServerCard server={mockServer} />)
    // counts are shown as numbers next to icons
    const ones = screen.getAllByText("1")
    expect(ones.length).toBeGreaterThanOrEqual(2)
  })

  it("renders repository source", () => {
    render(<ServerCard server={mockServer} />)
    expect(screen.getByText("github")).toBeInTheDocument()
  })

  it("renders the ownership display name instead of the subject", () => {
    const ownedServer: ServerResponse = {
      ...mockServer,
      _meta: {
        ...mockServer._meta,
        "aregistry.ai/ownership": {
          displayName: "Ada Lovelace",
          subject: "oidc-subject",
        },
      },
    }
    render(<ServerCard server={ownedServer} />)
    expect(screen.getByText("Ada Lovelace")).toBeInTheDocument()
    expect(screen.queryByText("oidc-subject")).not.toBeInTheDocument()
  })

  it("falls back to the ownership subject when the display name is empty", () => {
    const ownedServer: ServerResponse = {
      ...mockServer,
      _meta: {
        ...mockServer._meta,
        "aregistry.ai/ownership": {
          displayName: "",
          subject: "github-user",
        },
      },
    }
    render(<ServerCard server={ownedServer} />)
    expect(screen.getByText("github-user")).toBeInTheDocument()
  })

  it("renders a placeholder when ownership is absent", () => {
    render(<ServerCard server={mockServer} />)
    expect(screen.getByText("Unknown")).toBeInTheDocument()
  })

  it("does not render a last-modified element when updatedAt is absent", () => {
    const serverWithoutUpdatedAt: ServerResponse = {
      ...mockServer,
      _meta: {
        "io.modelcontextprotocol.registry/official": {
          publishedAt: "2024-11-01T00:00:00Z",
          status: "active",
          isLatest: true,
        },
      },
    }
    render(<ServerCard server={serverWithoutUpdatedAt} />)
    expect(screen.queryByText("Last modified")).not.toBeInTheDocument()
  })

  it("renders HTML in a display name as literal text", () => {
    const ownedServer: ServerResponse = {
      ...mockServer,
      _meta: {
        ...mockServer._meta,
        "aregistry.ai/ownership": {
          displayName: "<script>alert(1)</script>",
          subject: "oidc-subject",
        },
      },
    }
    const { container } = render(<ServerCard server={ownedServer} />)
    expect(screen.getByText("<script>alert(1)</script>")).toBeInTheDocument()
    expect(container.querySelector("script")).not.toBeInTheDocument()
  })

  it("falls back to name when title is not set", () => {
    const noTitle: ServerResponse = {
      server: { ...mockServer.server, title: undefined },
      _meta: {},
    }
    render(<ServerCard server={noTitle} />)
    expect(screen.getByText("acme/database-server")).toBeInTheDocument()
  })

  it("shows version count when provided", () => {
    render(<ServerCard server={mockServer} versionCount={5} />)
    expect(screen.getByText("+4")).toBeInTheDocument()
  })

  it("calls onClick when card is clicked", async () => {
    const onClick = vi.fn()
    render(<ServerCard server={mockServer} onClick={onClick} />)
    await userEvent.click(screen.getByText("Database Server"))
    expect(onClick).toHaveBeenCalledOnce()
  })

  it("shows deploy button when showDeploy is true and server has OCI package", () => {
    const onDeploy = vi.fn()
    const ociServer: ServerResponse = {
      server: { ...mockServer.server, packages: [{ registryType: "oci", identifier: "ghcr.io/acme/db", transport: { type: "stdio" } }] },
      _meta: mockServer._meta,
    }
    render(<ServerCard server={ociServer} showDeploy onDeploy={onDeploy} />)
    const btn = screen.getByText("Deploy").closest("button")!
    expect(btn).not.toBeDisabled()
  })

  it("renders all mutating controls when all capabilities are true", () => {
    renderWithCapabilities(serverWithCapabilities(
      { can_update: true, can_delete: true, can_deploy: true },
      [{ registryType: "oci", identifier: "ghcr.io/acme/db", transport: { type: "stdio" } }],
    ))

    expect(screen.getByRole("button", { name: "Edit server" })).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Remove server" })).toBeInTheDocument()
    expect(screen.getByRole("button", { name: /Deploy/i })).toBeInTheDocument()
  })

  it("renders only edit when update is allowed", () => {
    renderWithCapabilities(serverWithCapabilities({ can_update: true, can_delete: false, can_deploy: false }))

    expect(screen.getByRole("button", { name: "Edit server" })).toBeInTheDocument()
    expect(screen.queryByRole("button", { name: "Remove server" })).not.toBeInTheDocument()
    expect(screen.queryByRole("button", { name: /Deploy/i })).not.toBeInTheDocument()
  })

  it("renders no mutating controls when all capabilities are false", () => {
    renderWithCapabilities(serverWithCapabilities({ can_update: false, can_delete: false, can_deploy: false }))

    expect(screen.queryByRole("button", { name: "Edit server" })).not.toBeInTheDocument()
    expect(screen.queryByRole("button", { name: "Remove server" })).not.toBeInTheDocument()
    expect(screen.queryByRole("button", { name: /Deploy/i })).not.toBeInTheDocument()
  })

  it("renders no mutating controls when capabilities are absent", () => {
    renderWithCapabilities(serverWithCapabilities())

    expect(screen.queryByRole("button", { name: "Edit server" })).not.toBeInTheDocument()
    expect(screen.queryByRole("button", { name: "Remove server" })).not.toBeInTheDocument()
    expect(screen.queryByRole("button", { name: /Deploy/i })).not.toBeInTheDocument()
  })

  it("renders disabled deploy when permitted but no OCI package is available", () => {
    renderWithCapabilities(serverWithCapabilities({ can_update: false, can_delete: false, can_deploy: true }, []))

    expect(screen.getByRole("button", { name: /Deploy/i })).toBeDisabled()
  })

  it("hides deploy when not permitted even without an OCI package", () => {
    renderWithCapabilities(serverWithCapabilities({ can_update: false, can_delete: false, can_deploy: false }, []))

    expect(screen.queryByRole("button", { name: /Deploy/i })).not.toBeInTheDocument()
  })

  it("calls onDeploy without triggering onClick", async () => {
    const onDeploy = vi.fn()
    const onClick = vi.fn()
    const ociServer: ServerResponse = {
      server: { ...mockServer.server, packages: [{ registryType: "oci", identifier: "ghcr.io/acme/db", transport: { type: "stdio" } }] },
      _meta: mockServer._meta,
    }
    render(<ServerCard server={ociServer} showDeploy onDeploy={onDeploy} onClick={onClick} />)
    await userEvent.click(screen.getByText("Deploy"))
    expect(onDeploy).toHaveBeenCalledOnce()
    expect(onClick).not.toHaveBeenCalled()
  })

  it("renders edit button to the left of deploy", () => {
    const onEdit = vi.fn()
    const onDeploy = vi.fn()
    const ociServer: ServerResponse = {
      server: { ...mockServer.server, packages: [{ registryType: "oci", identifier: "ghcr.io/acme/db", transport: { type: "stdio" } }] },
      _meta: mockServer._meta,
    }
    render(<ServerCard server={ociServer} showEdit onEdit={onEdit} showDeploy onDeploy={onDeploy} />)

    const editButton = screen.getByRole("button", { name: "Edit server" })
    const deployButton = screen.getByRole("button", { name: /Deploy/i })

    expect(editButton.compareDocumentPosition(deployButton) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it("calls onEdit without triggering onClick", async () => {
    const onEdit = vi.fn()
    const onClick = vi.fn()
    render(<ServerCard server={mockServer} showEdit onEdit={onEdit} onClick={onClick} />)

    await userEvent.click(screen.getByRole("button", { name: "Edit server" }))

    expect(onEdit).toHaveBeenCalledOnce()
    expect(onClick).not.toHaveBeenCalled()
  })

  it("does not render edit button when callback is missing", () => {
    render(<ServerCard server={mockServer} showEdit />)
    expect(screen.queryByRole("button", { name: "Edit server" })).not.toBeInTheDocument()
  })

  it("disables deploy button when server has no OCI package", () => {
    const onDeploy = vi.fn()
    render(<ServerCard server={mockServer} showDeploy onDeploy={onDeploy} />)
    const btn = screen.getByText("Deploy").closest("button")!
    expect(btn).toBeDisabled()
  })

  it("does not trigger row click when disabled deploy wrapper is clicked", async () => {
    const onClick = vi.fn()
    render(<ServerCard server={mockServer} showDeploy onDeploy={vi.fn()} onClick={onClick} />)

    await userEvent.click(screen.getByText("Deploy").closest("span")!)

    expect(onClick).not.toHaveBeenCalled()
  })

  it("shows remove button when showDelete is true", () => {
    const onDelete = vi.fn()
    render(<ServerCard server={mockServer} showDelete onDelete={onDelete} />)
    expect(screen.getByRole("button", { name: "Remove server" })).toBeInTheDocument()
  })

  it("calls onDelete without triggering onClick", async () => {
    const onDelete = vi.fn()
    const onClick = vi.fn()
    render(<ServerCard server={mockServer} showDelete onDelete={onDelete} onClick={onClick} />)
    await userEvent.click(screen.getByRole("button", { name: "Remove server" }))
    expect(onDelete).toHaveBeenCalledOnce()
    expect(onClick).not.toHaveBeenCalled()
  })

  it("renders without optional fields", () => {
    const minimal: ServerResponse = {
      server: {
        $schema: "https://modelcontextprotocol.io/schemas/server.json",
        name: "test/minimal",
        description: "Bare minimum.",
        version: "0.0.1",
      },
      _meta: {},
    }
    render(<ServerCard server={minimal} />)
    expect(screen.getByText("Bare minimum.")).toBeInTheDocument()
    expect(screen.getByText("0.0.1")).toBeInTheDocument()
    expect(screen.getByText("te")).toBeInTheDocument()
  })

  it("renders initials instead of an image for an unsafe icon URL", () => {
    const unsafeIconServer: ServerResponse = {
      ...mockServer,
      server: {
        ...mockServer.server,
        icons: [{ src: "javascript:alert(1)" }],
      },
    }
    const { container } = render(<ServerCard server={unsafeIconServer} />)

    expect(screen.getByText("ac")).toBeInTheDocument()
    expect(container.querySelector("img")).not.toBeInTheDocument()
  })

  it("opens repository and website links without triggering row click", async () => {
    const onClick = vi.fn()
    const openSpy = vi.spyOn(window, "open").mockImplementation(() => null)
    render(<ServerCard server={mockServer} onClick={onClick} />)

    await userEvent.click(screen.getByRole("button", { name: "View repository" }))
    await userEvent.click(screen.getByRole("button", { name: "Visit website" }))

    expect(openSpy).toHaveBeenNthCalledWith(1, "https://github.com/acme/database-server", "_blank")
    expect(openSpy).toHaveBeenNthCalledWith(2, "https://acme.dev/database-server", "_blank")
    expect(onClick).not.toHaveBeenCalled()
    openSpy.mockRestore()
  })

  it("does not render actions for javascript URLs", () => {
    const unsafeServer: ServerResponse = {
      ...mockServer,
      server: {
        ...mockServer.server,
        repository: { url: "javascript:alert(1)", source: "github" },
        websiteUrl: "javascript:alert(1)",
      },
    }
    render(<ServerCard server={unsafeServer} />)
    expect(screen.queryByRole("button", { name: "View repository" })).not.toBeInTheDocument()
    expect(screen.queryByRole("button", { name: "Visit website" })).not.toBeInTheDocument()
  })

  it("falls back to raw date string when date formatting throws", () => {
    const dateSpy = vi.spyOn(Date.prototype, "toLocaleDateString").mockImplementation(() => {
      throw new Error("format failed")
    })
    render(<ServerCard server={mockServer} />)
    expect(screen.getByText("2024-11-01T00:00:00Z")).toBeInTheDocument()
    dateSpy.mockRestore()
  })

  it("renders icon, verification badges, and star metadata when present", () => {
    const richServer: ServerResponse = {
      server: {
        ...mockServer.server,
        icons: [{ src: "https://cdn.acme.dev/icon.png" }],
        _meta: {
          "io.modelcontextprotocol.registry/publisher-provided": {
            "aregistry.ai/metadata": {
              stars: 1234,
              identity: {
                org_is_verified: true,
                publisher_identity_verified_by_jwt: true,
              },
            },
          },
        },
      },
      _meta: mockServer._meta,
    }

    const { container } = render(<ServerCard server={richServer} />)

    expect(container.querySelector('img[src="https://cdn.acme.dev/icon.png"]')).toBeInTheDocument()
    expect(screen.getByText("1,234")).toBeInTheDocument()
  })
})
