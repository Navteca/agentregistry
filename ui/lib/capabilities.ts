import type { CapabilitiesMeta } from "@/lib/api/types.gen"

export type ArtifactControlFlags = {
  showEdit: boolean
  showDelete: boolean
  showDeploy: boolean
  showReview: boolean
  showOverride: boolean
}

export function capabilityFlags(capabilities?: Partial<CapabilitiesMeta>): ArtifactControlFlags {
  return {
    showEdit: capabilities?.can_update === true,
    showDelete: capabilities?.can_delete === true,
    showDeploy: capabilities?.can_deploy === true,
    showReview: capabilities?.can_review === true,
    showOverride: capabilities?.can_override === true,
  }
}
