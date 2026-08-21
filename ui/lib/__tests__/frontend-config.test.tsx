import { render } from "@testing-library/react"
import { describe, expect, it } from "vitest"

import { useFrontendConfig } from "../frontend-config"

function ConfigConsumer() {
  useFrontendConfig()
  return null
}

describe("useFrontendConfig", () => {
  it("throws when rendered outside a provider", () => {
    expect(() => render(<ConfigConsumer />)).toThrow(
      "useFrontendConfig must be used within a FrontendConfigProvider",
    )
  })
})
