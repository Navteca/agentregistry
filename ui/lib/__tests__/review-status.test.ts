import { describe, expect, it } from "vitest"

import { overallStatusPresentation } from "../review-status"

describe("overallStatusPresentation", () => {
  it.each([
    ["certified", "detail", "Certified"],
    ["rejected", "detail", "Rejected"],
    ["pending", "detail", "Pending"],
    ["certified", "card", "Certified"],
    ["rejected", "card", "Rejected"],
    ["pending", "card", "Pending Review"],
  ] as const)("maps %s in the %s variant to %s", (status, variant, label) => {
    expect(overallStatusPresentation(status, variant).label).toBe(label)
  })

  it.each([
    ["detail", "Pending"],
    ["card", "Pending Review"],
  ] as const)("falls back to %s for an unrecognised status in the %s variant", (variant, label) => {
    expect(overallStatusPresentation("unknown", variant).label).toBe(label)
  })
})
