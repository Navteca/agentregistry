"use client"

import { createContext, useContext, type ReactNode } from "react"

export interface FrontendConfig {
  keycloak_url: string
  keycloak_realm: string
  keycloak_client_id: string
  api_base_url?: string
  gateway_base_url?: string
  anonymous_auth_enabled?: boolean
  show_github_link?: boolean
  show_discord_link?: boolean
  review_types?: string[]
  review_outcomes?: string[]
  review_failure_outcome?: string
  review_override_outcome?: string
}

const FrontendConfigContext = createContext<FrontendConfig | null>(null)

export function FrontendConfigProvider({
  config,
  children,
}: {
  config: FrontendConfig
  children: ReactNode
}) {
  return <FrontendConfigContext.Provider value={config}>{children}</FrontendConfigContext.Provider>
}

export function useFrontendConfig() {
  return useContext(FrontendConfigContext)
}
