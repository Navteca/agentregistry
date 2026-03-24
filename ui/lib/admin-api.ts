// Auto-generated API client configuration.
// Types and SDK functions are generated from the OpenAPI spec.
// Regenerate with: make gen-client

import { client } from './api/client.gen'

export const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || (typeof window !== 'undefined' && window.location.origin) || ''

const AUTH_HEADER_NAME = 'Authorization'

client.setConfig({ baseUrl: API_BASE_URL })

export function getApiBaseUrl(): string {
	return (client.getConfig().baseUrl as string) || API_BASE_URL
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
