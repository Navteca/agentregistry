import type Keycloak from "keycloak-js"
import { clearRegistryAuthToken } from "@/lib/admin-api"

export interface KeycloakUserProfile {
  name: string
  email?: string
  username?: string
  initials: string
}

let activeKeycloak: Keycloak | null = null
let activeUser: KeycloakUserProfile | null = null
const sessionListeners = new Set<() => void>()

function notifySessionListeners(): void {
  for (const listener of sessionListeners) {
    listener()
  }
}

function readClaim(value: unknown): string | undefined {
  return typeof value === "string" && value.trim() ? value.trim() : undefined
}

function toInitials(displayName: string): string {
  const words = displayName
    .split(/\s+/)
    .map((word) => word.trim())
    .filter(Boolean)

  if (words.length === 0) {
    return "U"
  }

  if (words.length === 1) {
    return words[0].slice(0, 2).toUpperCase()
  }

  return `${words[0][0] ?? ""}${words[1][0] ?? ""}`.toUpperCase()
}

function extractUserProfile(instance: Keycloak | null): KeycloakUserProfile | null {
  if (!instance?.authenticated || !instance.tokenParsed) {
    return null
  }

  const tokenParsed = instance.tokenParsed as Record<string, unknown>
  const name =
    readClaim(tokenParsed.name) ??
    readClaim(tokenParsed.preferred_username) ??
    readClaim(tokenParsed.email)

  if (!name) {
    return null
  }

  const email = readClaim(tokenParsed.email)
  const username = readClaim(tokenParsed.preferred_username)

  return {
    name,
    email,
    username,
    initials: toInitials(name),
  }
}

export function getActiveUserProfile(): KeycloakUserProfile | null {
  return activeUser
}

export function subscribeKeycloakSession(listener: () => void): () => void {
  sessionListeners.add(listener)
  return () => {
    sessionListeners.delete(listener)
  }
}

export function setActiveKeycloakInstance(instance: Keycloak | null): void {
  activeKeycloak = instance
  activeUser = extractUserProfile(instance)
  notifySessionListeners()
}

export function refreshActiveUserProfile(): void {
  activeUser = extractUserProfile(activeKeycloak)
  notifySessionListeners()
}

export async function logoutFromKeycloak(): Promise<void> {
  const keycloak = activeKeycloak
  clearRegistryAuthToken()
  setActiveKeycloakInstance(null)

  if (!keycloak) {
    if (typeof window !== "undefined") {
      window.location.reload()
    }
    return
  }

  await keycloak.logout({
    redirectUri: typeof window !== "undefined" ? window.location.origin : undefined,
  })
}
