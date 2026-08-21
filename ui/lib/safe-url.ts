export function getSafeHttpUrl(value: unknown): string | null {
  if (typeof value !== "string") {
    return null
  }

  const candidate = value.trim()
  if (!candidate) {
    return null
  }

  try {
    const parsed = new URL(candidate)
    return parsed.protocol === "http:" || parsed.protocol === "https:" ? candidate : null
  } catch {
    return null
  }
}
