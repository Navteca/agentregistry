import { describe, expect, it } from "vitest"
import { capabilityFlags } from "../capabilities"

describe("capabilityFlags", () => {
  it("enables all controls when all capabilities are true", () => {
    expect(capabilityFlags({ can_update: true, can_delete: true, can_deploy: true, can_review: true })).toEqual({
      showEdit: true,
      showDelete: true,
      showDeploy: true,
      showReview: true,
    })
  })

  it("disables all controls when all capabilities are false", () => {
    expect(capabilityFlags({ can_update: false, can_delete: false, can_deploy: false })).toEqual({
      showEdit: false,
      showDelete: false,
      showDeploy: false,
      showReview: false,
    })
  })

  it("maps mixed capabilities independently", () => {
    expect(capabilityFlags({ can_update: true, can_delete: false, can_deploy: true })).toEqual({
      showEdit: true,
      showDelete: false,
      showDeploy: true,
      showReview: false,
    })
  })

  it("hides all controls when capabilities are undefined", () => {
    expect(capabilityFlags(undefined)).toEqual({
      showEdit: false,
      showDelete: false,
      showDeploy: false,
      showReview: false,
    })
  })

  it("hides all controls for an empty metadata object", () => {
    expect(capabilityFlags({})).toEqual({
      showEdit: false,
      showDelete: false,
      showDeploy: false,
      showReview: false,
    })
  })

  it("requires each capability field to be explicitly true", () => {
    expect(capabilityFlags({ can_update: true, can_delete: true })).toEqual({
      showEdit: true,
      showDelete: true,
      showDeploy: false,
      showReview: false,
    })
  })

  it("enables review only when can_review is explicitly true", () => {
    expect(capabilityFlags({ can_review: true }).showReview).toBe(true)
    expect(capabilityFlags({ can_review: false }).showReview).toBe(false)
  })
})
