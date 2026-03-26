"use client"

import { type ReactNode, useEffect, useRef, useState } from "react"
import Keycloak from "keycloak-js"
import { clearRegistryAuthToken, getApiBaseUrl, setApiBaseUrl, setGatewayBaseUrl, setRegistryAuthToken } from "@/lib/admin-api"
import { refreshActiveUserProfile, setActiveKeycloakInstance } from "@/lib/keycloak-session"

type GateStatus = "loading" | "ready" | "error"

interface GateState {
  status: GateStatus
  message?: string
}

interface OIDCExchangeResponse {
  registry_token: string
  expires_at: number
}

const REFRESH_INTERVAL_MS = 30_000
const REGISTRY_REFRESH_SKEW_SECONDS = 60
const OIDC_DISCOVERY_TIMEOUT_MS = 8_000

interface FrontendConfig {
  keycloak_url: string
  keycloak_realm: string
  keycloak_client_id: string
  api_base_url?: string
  gateway_base_url?: string
}

async function fetchFrontendConfig(): Promise<FrontendConfig> {
  // Always use a relative URL here to bootstrap config from the same host.
  // This prevents stale compiled API URLs from hijacking initial requests.
  const response = await fetch(`/v0/config/frontend`)
  if (!response.ok) {
    throw new Error(`Failed to fetch frontend config (${response.status})`)
  }
  const cfg = (await response.json()) as FrontendConfig
  const missing: string[] = []
  if (!cfg.keycloak_url) missing.push("KEYCLOAK_URL")
  if (!cfg.keycloak_realm) missing.push("KEYCLOAK_REALM")
  if (!cfg.keycloak_client_id) missing.push("KEYCLOAK_CLIENT_ID")
  if (missing.length > 0) {
    throw new Error(`Missing frontend OIDC config: ${missing.join(", ")}`)
  }
  return cfg
}

export function AuthGate({ children }: { children: ReactNode }) {
  const [state, setState] = useState<GateState>({ status: "loading" })
  const keycloakRef = useRef<Keycloak | null>(null)
  const registryExpiresAtRef = useRef<number>(0)

  useEffect(() => {
    let cancelled = false
    let refreshTimer: number | null = null

    const exchangeOIDCToken = async (idToken: string) => {
      const response = await fetch(`${getApiBaseUrl()}/v0/auth/oidc`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ oidc_token: idToken }),
      })

      if (!response.ok) {
        const responseText = await response.text()
        throw new Error(`OIDC token exchange failed (${response.status}): ${responseText}`)
      }

      const payload = (await response.json()) as OIDCExchangeResponse
      if (!payload.registry_token || !payload.expires_at) {
        throw new Error("OIDC exchange response is missing required fields")
      }

      setRegistryAuthToken(payload.registry_token)
      registryExpiresAtRef.current = payload.expires_at
    }

    const refreshAuthIfNeeded = async () => {
      const keycloak = keycloakRef.current
      if (!keycloak) {
        return
      }

      if (!keycloak.authenticated || !keycloak.idToken) {
        await keycloak.login()
        return
      }

      const keycloakTokenUpdated = await keycloak.updateToken(90)
      if (keycloakTokenUpdated) {
        refreshActiveUserProfile()
      }
      const now = Math.floor(Date.now() / 1000)
      const secondsUntilRegistryExpiry = registryExpiresAtRef.current - now
      const registryExpiresSoon = secondsUntilRegistryExpiry <= REGISTRY_REFRESH_SKEW_SECONDS

      if (keycloakTokenUpdated || registryExpiresSoon) {
        if (!keycloak.idToken) {
          throw new Error("id_token not available after refresh")
        }
        await exchangeOIDCToken(keycloak.idToken)
      }
    }

    const initAuth = async () => {
      try {
        const cfg = await fetchFrontendConfig()
        if (cfg.api_base_url) {
          setApiBaseUrl(cfg.api_base_url)
        }
        if (cfg.gateway_base_url) {
          setGatewayBaseUrl(cfg.gateway_base_url)
        }

        const keycloakUrl = cfg.keycloak_url
        const keycloakRealm = cfg.keycloak_realm
        const discoveryUrl = `${keycloakUrl}/realms/${keycloakRealm}/.well-known/openid-configuration`

        const discoveryController = new AbortController()
        const discoveryTimeout = window.setTimeout(() => {
          discoveryController.abort()
        }, OIDC_DISCOVERY_TIMEOUT_MS)

        try {
          const discoveryResponse = await fetch(discoveryUrl, {
            signal: discoveryController.signal,
          })
          if (!discoveryResponse.ok) {
            throw new Error(`Discovery endpoint returned ${discoveryResponse.status}`)
          }
        } catch {
          throw new Error(
            `Cannot reach Keycloak discovery at ${discoveryUrl}. Ensure port-forward is running and keycloak.default.svc.cluster.local resolves locally (for example via /etc/hosts).`,
          )
        } finally {
          window.clearTimeout(discoveryTimeout)
        }

        const keycloak = new Keycloak({
          url: keycloakUrl,
          realm: keycloakRealm,
          clientId: cfg.keycloak_client_id,
        })
        keycloakRef.current = keycloak
        setActiveKeycloakInstance(keycloak)

        const authenticated = await keycloak.init({
          onLoad: "login-required",
          pkceMethod: "S256",
          checkLoginIframe: false,
        })

        if (!authenticated || !keycloak.idToken) {
          await keycloak.login()
          return
        }

        refreshActiveUserProfile()

        await exchangeOIDCToken(keycloak.idToken)

        if (cancelled) {
          return
        }

        setState({ status: "ready" })

        refreshTimer = window.setInterval(() => {
          void refreshAuthIfNeeded().catch((err) => {
            console.error("auth refresh failed", err)
            setActiveKeycloakInstance(null)
            clearRegistryAuthToken()
            if (!cancelled) {
              setState({
                status: "error",
                message: err instanceof Error ? err.message : "Authentication refresh failed",
              })
            }
          })
        }, REFRESH_INTERVAL_MS)
      } catch (err) {
        setActiveKeycloakInstance(null)
        clearRegistryAuthToken()
        if (!cancelled) {
          setState({
            status: "error",
            message: err instanceof Error ? err.message : "Authentication failed",
          })
        }
      }
    }

    void initAuth()

    return () => {
      cancelled = true
      if (refreshTimer !== null) {
        window.clearInterval(refreshTimer)
      }
      setActiveKeycloakInstance(null)
      clearRegistryAuthToken()
    }
  }, [])

  if (state.status === "ready") {
    return <>{children}</>
  }

  if (state.status === "error") {
    return (
      <main className="min-h-screen bg-background flex items-center justify-center p-6">
        <div className="max-w-xl w-full rounded-lg border bg-card p-6 space-y-4">
          <h1 className="text-xl font-semibold">Authentication failed</h1>
          <p className="text-sm text-muted-foreground break-words">
            {state.message ?? "Unable to authenticate with Keycloak."}
          </p>
          <button
            type="button"
            onClick={() => window.location.reload()}
            className="inline-flex items-center rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
          >
            Retry
          </button>
        </div>
      </main>
    )
  }

  return (
    <main className="min-h-screen bg-background flex items-center justify-center p-6">
      <div className="text-sm text-muted-foreground">Authenticating with Keycloak...</div>
    </main>
  )
}
