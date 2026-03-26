// Auto-generated API client configuration.
// Types and SDK functions are generated from the OpenAPI spec.
// Regenerate with: make gen-client

import { client } from './api/client.gen'

const COMPILED_API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || (typeof window !== 'undefined' && window.location.origin) || ''
const DEFAULT_GATEWAY_BASE_URL = process.env.NEXT_PUBLIC_GATEWAY_URL || 'http://localhost:21212'

const AUTH_HEADER_NAME = 'Authorization'
let gatewayBaseUrl = DEFAULT_GATEWAY_BASE_URL

client.setConfig({ baseUrl: COMPILED_API_BASE_URL })

export function getApiBaseUrl(): string {
	return (client.getConfig().baseUrl as string) || COMPILED_API_BASE_URL
}

export function setApiBaseUrl(baseUrl: string): void {
	const normalized = baseUrl.trim().replace(/\/+$/, '')
	if (!normalized) {
		return
	}
	client.setConfig({ baseUrl: normalized })
}

export function getGatewayBaseUrl(): string {
	return gatewayBaseUrl
}

export function setGatewayBaseUrl(baseUrl: string): void {
	const normalized = baseUrl.trim().replace(/\/+$/, '')
	if (!normalized) {
		return
	}
	gatewayBaseUrl = normalized
}

export function setRegistryAuthToken(token: string): void {
	client.setConfig({
		headers: {
			[AUTH_HEADER_NAME]: `Bearer ${token}`,
		},
	})
}

export function clearRegistryAuthToken(): void {
	client.setConfig({
		headers: {
			[AUTH_HEADER_NAME]: null,
		},
	})
}

export function getRegistryAuthHeader(): string | null {
	const configuredHeaders = client.getConfig().headers
	if (!configuredHeaders) {
		return null
	}

	if (configuredHeaders instanceof Headers) {
		return configuredHeaders.get(AUTH_HEADER_NAME)
	}

	if (Array.isArray(configuredHeaders)) {
		return new Headers(configuredHeaders).get(AUTH_HEADER_NAME)
	}

	const value = configuredHeaders[AUTH_HEADER_NAME]
	return typeof value === 'string' ? value : null
}

export { client }
export * from './api/sdk.gen'
export * from './api/types.gen'
