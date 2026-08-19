"use client"

import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { ChevronDown, ChevronUp, HelpCircle } from "lucide-react"
import type { Review } from "@/lib/admin-api"
import type { StatusPresentation } from "@/lib/review-status"

type ReviewTypeCardProps = {
  reviewType: string
  statusPresentation?: StatusPresentation
  reviews: Review[]
  unconfigured?: boolean
  expanded: boolean
  onToggle: () => void
  canOverride?: boolean
  failureOutcome?: string
  overriddenReviewIDs?: ReadonlySet<number>
  overrideTargetID?: number
  overrideReason: string
  overrideError?: string
  overrideSubmitting?: boolean
  onStartOverride?: (review: Review) => void
  onOverrideReasonChange?: (reason: string) => void
  onCancelOverride?: () => void
  onSubmitOverride?: (review: Review) => void
}

function formatDate(dateString: string): string {
  const date = new Date(dateString)
  if (Number.isNaN(date.getTime())) {
    return dateString
  }
  return date.toLocaleString("en-US", {
    year: "numeric",
    month: "long",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  })
}

function ReviewStateBadge({
  label,
  tooltip,
}: {
  label: string
  tooltip: string
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="inline-flex items-center gap-1 rounded-full border border-border px-2 py-0.5 text-muted-foreground">
          {label}
          <HelpCircle className="h-3 w-3" aria-hidden="true" />
        </span>
      </TooltipTrigger>
      <TooltipContent>
        <p>{tooltip}</p>
      </TooltipContent>
    </Tooltip>
  )
}

function ReviewFinding({
  review,
  canOverride,
  failureOutcome,
  overriddenReviewIDs,
  overrideTargetID,
  overrideReason,
  overrideError,
  overrideSubmitting,
  onStartOverride,
  onOverrideReasonChange,
  onCancelOverride,
  onSubmitOverride,
}: Omit<ReviewTypeCardProps, "reviewType" | "statusPresentation" | "reviews" | "unconfigured" | "expanded" | "onToggle"> & {
  review: Review
}) {
  const canRecordOverride =
    canOverride === true &&
    review.overrides_review_id === undefined &&
    review.outcome === failureOutcome &&
    review.is_current === true &&
    review.is_stale !== true &&
    review.is_superseded !== true &&
    !overriddenReviewIDs?.has(review.id)
  const isOverrideTarget = overrideTargetID === review.id

  return (
    <article
      data-testid={`review-finding-${review.id}`}
      className="rounded-md border border-border bg-muted/20 p-3"
    >
      <div className="flex flex-wrap items-center gap-2 text-xs">
        <span className="rounded-full border px-2 py-0.5">
          {review.overrides_review_id !== undefined ? "Override" : review.outcome}
        </span>
        {review.overrides_review_id !== undefined && (
          <span className="text-muted-foreground">Overrides review #{review.overrides_review_id}</span>
        )}
        {review.is_current === true && (
          <ReviewStateBadge
            label="Current"
            tooltip="The artifact hasn't changed since this review."
          />
        )}
        {review.is_superseded === true && (
          <ReviewStateBadge
            label="Superseded"
            tooltip="This reviewer has since submitted a newer review."
          />
        )}
        {review.is_stale === true && (
          <ReviewStateBadge
            label="Stale"
            tooltip="The artifact has changed since this review."
          />
        )}
      </div>
      <p className="mt-2 text-xs text-muted-foreground">
        <span>{review.reviewer_display_name || "Unknown reviewer"}</span>
        <span aria-hidden="true"> · </span>
        <time dateTime={review.created_at}>{formatDate(review.created_at)}</time>
      </p>
      <p className="mt-2 whitespace-pre-wrap break-words text-sm">{review.notes}</p>
      {canRecordOverride && !isOverrideTarget && (
        <Button
          type="button"
          variant="outline"
          className="mt-3"
          onClick={() => onStartOverride?.(review)}
        >
          Override failed review
        </Button>
      )}
      {isOverrideTarget && (
        <div className="mt-3 space-y-2 rounded-md border border-dashed p-3">
          <label htmlFor={`override-reason-${review.id}`} className="text-sm font-medium">
            Override reason
          </label>
          <Textarea
            id={`override-reason-${review.id}`}
            value={overrideReason}
            onChange={(event) => onOverrideReasonChange?.(event.target.value)}
            placeholder="Explain why this finding is being overridden..."
            disabled={overrideSubmitting}
            required
          />
          {overrideError && (
            <p role="alert" className="text-sm text-red-600 dark:text-red-400">
              {overrideError}
            </p>
          )}
          <div className="flex gap-2">
            <Button
              type="button"
              disabled={overrideSubmitting || !overrideReason.trim()}
              onClick={() => onSubmitOverride?.(review)}
            >
              {overrideSubmitting ? "Recording..." : "Record override"}
            </Button>
            <Button type="button" variant="ghost" disabled={overrideSubmitting} onClick={onCancelOverride}>
              Cancel
            </Button>
          </div>
        </div>
      )}
    </article>
  )
}

export function ReviewTypeCard({
  reviewType,
  statusPresentation,
  reviews,
  unconfigured = false,
  expanded,
  onToggle,
  canOverride,
  failureOutcome,
  overriddenReviewIDs,
  overrideTargetID,
  overrideReason,
  overrideError,
  overrideSubmitting,
  onStartOverride,
  onOverrideReasonChange,
  onCancelOverride,
  onSubmitOverride,
}: ReviewTypeCardProps) {
  const sanitizedReviewType = reviewType.replace(/[^a-zA-Z0-9_-]+/g, "-")
  const cardId = `review-type-${sanitizedReviewType}`

  return (
    <article data-testid={`review-type-card-${sanitizedReviewType}`} className="rounded-md border p-3">
      <div className="flex items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2">
          <h3 className="truncate text-sm font-medium">{reviewType}</h3>
          {unconfigured ? (
            <span className="rounded-full border border-border px-2 py-0.5 text-xs text-muted-foreground">
              No longer configured
            </span>
          ) : (
            <span className={`rounded-full border px-2 py-0.5 text-xs font-semibold ${statusPresentation?.className}`}>
              {statusPresentation?.label}
            </span>
          )}
        </div>
        {reviews.length > 0 ? (
          <Button
            type="button"
            variant="ghost"
            className="h-auto shrink-0 gap-2 px-2 py-1.5 text-xs text-muted-foreground"
            aria-label={`${reviews.length} review${reviews.length === 1 ? "" : "s"} for ${reviewType}`}
            aria-expanded={expanded}
            aria-controls={cardId}
            onClick={onToggle}
          >
            <span>{reviews.length} review{reviews.length === 1 ? "" : "s"}</span>
            {expanded ? (
              <ChevronUp className="h-4 w-4" aria-hidden="true" />
            ) : (
              <ChevronDown className="h-4 w-4" aria-hidden="true" />
            )}
          </Button>
        ) : (
          <span className="shrink-0 text-xs text-muted-foreground">No reviews yet</span>
        )}
      </div>
      {reviews.length > 0 && expanded && (
        <div id={cardId} className="mt-3 space-y-3">
          {reviews.map((review) => (
            <ReviewFinding
              key={review.id}
              review={review}
              canOverride={canOverride}
              failureOutcome={failureOutcome}
              overriddenReviewIDs={overriddenReviewIDs}
              overrideTargetID={overrideTargetID}
              overrideReason={overrideReason}
              overrideError={overrideError}
              overrideSubmitting={overrideSubmitting}
              onStartOverride={onStartOverride}
              onOverrideReasonChange={onOverrideReasonChange}
              onCancelOverride={onCancelOverride}
              onSubmitOverride={onSubmitOverride}
            />
          ))}
        </div>
      )}
    </article>
  )
}
