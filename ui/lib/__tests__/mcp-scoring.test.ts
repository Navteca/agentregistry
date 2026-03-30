import { describe, expect, it, vi } from "vitest"
import { buildScorePath, triggerMcpScoringForCreatedServer } from "../mcp-scoring"

describe("mcp-scoring integration helper", () => {
  it("builds score path when name and version are present", () => {
    const path = buildScorePath({
      name: "io.github.navteca/hello mcp",
      version: "1.0.0+build/meta",
    })
    expect(path).toEqual({
      serverName: "io.github.navteca/hello mcp",
      version: "1.0.0+build/meta",
    })
  })

  it("returns null when server is missing", () => {
    expect(buildScorePath(undefined)).toBeNull()
    expect(buildScorePath(null)).toBeNull()
  })

  it("returns null when name is missing", () => {
    expect(buildScorePath({ version: "1.0.0" })).toBeNull()
    expect(buildScorePath({ name: "   ", version: "1.0.0" })).toBeNull()
  })

  it("returns null when version is missing", () => {
    expect(buildScorePath({ name: "io.github/navteca" })).toBeNull()
    expect(buildScorePath({ name: "io.github/navteca", version: "   " })).toBeNull()
  })

  it("calls scoreServer and success callback when scoring succeeds", async () => {
    const scoreServer = vi.fn().mockResolvedValue({})
    const onSuccess = vi.fn()
    const onFailure = vi.fn()

    const result = await triggerMcpScoringForCreatedServer({
      server: { name: "io.github/navteca", version: "1.0.0" },
      scoreServer,
      onSuccess,
      onFailure,
    })

    expect(result).toBe(true)
    expect(scoreServer).toHaveBeenCalledTimes(1)
    expect(scoreServer).toHaveBeenCalledWith({
      path: {
        serverName: "io.github/navteca",
        version: "1.0.0",
      },
    })
    expect(onSuccess).toHaveBeenCalledTimes(1)
    expect(onFailure).not.toHaveBeenCalled()
  })

  it("calls failure callback when scoring fails", async () => {
    const scoreServer = vi.fn().mockRejectedValue(new Error("down"))
    const onSuccess = vi.fn()
    const onFailure = vi.fn()

    const result = await triggerMcpScoringForCreatedServer({
      server: { name: "io.github/navteca", version: "1.0.0" },
      scoreServer,
      onSuccess,
      onFailure,
    })

    expect(result).toBe(false)
    expect(scoreServer).toHaveBeenCalledTimes(1)
    expect(onSuccess).not.toHaveBeenCalled()
    expect(onFailure).toHaveBeenCalledTimes(1)
  })

  it("does nothing when score path cannot be built", async () => {
    const scoreServer = vi.fn()
    const onSuccess = vi.fn()
    const onFailure = vi.fn()

    const result = await triggerMcpScoringForCreatedServer({
      server: { name: "", version: "1.0.0" },
      scoreServer,
      onSuccess,
      onFailure,
    })

    expect(result).toBe(false)
    expect(scoreServer).not.toHaveBeenCalled()
    expect(onSuccess).not.toHaveBeenCalled()
    expect(onFailure).not.toHaveBeenCalled()
  })
})
