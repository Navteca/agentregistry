export type ScorePath = {
  serverName: string
  version: string
}

export type CreatedServerIdentity = {
  name?: string
  version?: string
} | null | undefined

export type ScoreServerFn = (input: { path: ScorePath }) => Promise<unknown>

export function buildScorePath(server: CreatedServerIdentity): ScorePath | null {
  const name = server?.name?.trim()
  const version = server?.version?.trim()
  if (!name || !version) {
    return null
  }
  return {
    serverName: encodeURIComponent(name),
    version: encodeURIComponent(version),
  }
}

export async function triggerMcpScoringForCreatedServer(args: {
  server: CreatedServerIdentity
  scoreServer: ScoreServerFn
  onSuccess: () => void
  onFailure: () => void
}): Promise<boolean> {
  const path = buildScorePath(args.server)
  if (!path) {
    return false
  }

  try {
    await args.scoreServer({ path })
    args.onSuccess()
    return true
  } catch {
    args.onFailure()
    return false
  }
}
