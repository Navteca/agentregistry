"use client"

import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { ChevronDown, ChevronUp } from "lucide-react"
import {
  createReviewV0,
  getFrontendConfig,
  listReviewsV0,
  type CapabilitiesMeta,
  type FrontendConfigBody,
  type Review,
  type ReviewSummary,
} from "@/lib/admin-api"
import { capabilityFlags } from "@/lib/capabilities"

interface ReviewSectionProps {
  artifactType: string
  artifactName: string
  artifactVersion: string
  reviewSummary?: ReviewSummary
  capabilities?: Partial<CapabilitiesMeta>
  onReviewSubmitted?: () => void | Promise<void>
}

type StatusPresentation = {
  label: string
  className: string
}

const overallStatusPresentation = (status: string): StatusPresentation => {
  switch (status) {
    case "certified":
      return {
        label: "Certified",
        className: "border-green-500/30 bg-green-500/5 text-green-700 dark:text-green-400",
      }
    case "rejected":
      return {
        label: "Rejected",
        className: "border-red-500/30 bg-red-500/5 text-red-700 dark:text-red-400",
      }
    default:
      return {
        label: "Pending",
        className: "border-amber-500/30 bg-amber-500/5 text-amber-700 dark:text-amber-400",
      }
  }
}

const perTypeStatusPresentation = (status: string): StatusPresentation => {
  switch (status) {
    case "pass":
      return {
        label: "Passed",
        className: "border-green-500/30 bg-green-500/5 text-green-700 dark:text-green-400",
      }
    case "fail":
      return {
        label: "Failed",
        className: "border-red-500/30 bg-red-500/5 text-red-700 dark:text-red-400",
      }
    default:
      return {
        label: "Pending",
        className: "border-amber-500/30 bg-amber-500/5 text-amber-700 dark:text-amber-400",
      }
  }
}

function extractErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) {
    return error.message
  }
  if (typeof error === "string" && error.trim()) {
    return error
  }
  if (error && typeof error === "object") {
    const record = error as Record<string, unknown>
    for (const key of ["detail", "message", "title"]) {
      if (typeof record[key] === "string" && record[key]) {
        return record[key] as string
      }
    }
  }
  return fallback
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

export function ReviewSection({
  artifactType,
  artifactName,
  artifactVersion,
  reviewSummary,
  capabilities,
  onReviewSubmitted,
}: ReviewSectionProps) {
  const [reviews, setReviews] = useState<Review[]>([])
  const [frontendConfig, setFrontendConfig] = useState<FrontendConfigBody | null>(null)
  const [reviewsLoading, setReviewsLoading] = useState(true)
  const [reviewsError, setReviewsError] = useState<string | null>(null)
  const [configError, setConfigError] = useState<string | null>(null)
  const [reviewType, setReviewType] = useState("")
  const [outcome, setOutcome] = useState("")
  const [notes, setNotes] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const [submissionError, setSubmissionError] = useState<string | null>(null)
  const [showEarlierReviews, setShowEarlierReviews] = useState(false)
  const reviewsRequest = useRef(0)

  const { showReview } = capabilityFlags(capabilities)

  const loadReviews = useCallback(async () => {
    const requestId = reviewsRequest.current + 1
    reviewsRequest.current = requestId
    setReviewsLoading(true)
    setReviewsError(null)
    try {
      const { data } = await listReviewsV0({
        path: {
          artifactType,
          artifactName,
          version: artifactVersion,
        },
        throwOnError: true,
      })
      if (reviewsRequest.current === requestId) {
        setReviews(data)
      }
    } catch (error) {
      if (reviewsRequest.current === requestId) {
        setReviews([])
        setReviewsError(extractErrorMessage(error, "Unable to load review findings"))
      }
    } finally {
      if (reviewsRequest.current === requestId) {
        setReviewsLoading(false)
      }
    }
  }, [artifactName, artifactType, artifactVersion])

  useEffect(() => {
    let cancelled = false
    setConfigError(null)
    getFrontendConfig({ throwOnError: true })
      .then(({ data }) => {
        if (!cancelled) {
          setFrontendConfig(data)
        }
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setFrontendConfig(null)
          setConfigError(extractErrorMessage(error, "Unable to load review options"))
        }
      })
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    void loadReviews()
  }, [loadReviews])

  useEffect(() => {
    const types = frontendConfig?.review_types ?? []
    const outcomes = frontendConfig?.review_outcomes ?? []
    setReviewType((current) => (types.includes(current) ? current : (types[0] ?? "")))
    setOutcome((current) => (outcomes.includes(current) ? current : (outcomes[0] ?? "")))
  }, [frontendConfig])

  const statusTypes = useMemo(() => {
    const summaryTypes = reviewSummary?.per_type ?? []
    return summaryTypes.map((type) => ({
      reviewType: type.review_type,
      status: type.status,
      outcome: type.outcome,
      reviewerDisplayNames: type.reviewer_display_names ?? [],
    }))
  }, [reviewSummary])

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setSubmissionError(null)
    if (!reviewType || !outcome) {
      const message = "Review type and outcome are required"
      setSubmissionError(message)
      toast.error(message)
      return
    }
    if (!notes.trim()) {
      const message = "Findings are required"
      setSubmissionError(message)
      toast.error(message)
      return
    }

    setSubmitting(true)
    try {
      await createReviewV0({
        path: {
          artifactType,
          artifactName,
          version: artifactVersion,
        },
        body: {
          review_type: reviewType,
          outcome,
          notes: notes.trim(),
        },
        throwOnError: true,
      })
      toast.success("Review submitted successfully")
      setNotes("")
      await loadReviews()
      await onReviewSubmitted?.()
    } catch (error) {
      const message = extractErrorMessage(error, "Unable to submit review")
      setSubmissionError(message)
      toast.error(message)
    } finally {
      setSubmitting(false)
    }
  }

  const overallStatus = overallStatusPresentation(reviewSummary?.status ?? "pending")
  const currentReviews = reviews.filter((review) => review.is_current === true)
  const earlierReviews = reviews.filter((review) => review.is_current !== true)
  const displayedReviews = showEarlierReviews ? reviews : currentReviews

  return (
    <TooltipProvider>
      <section aria-labelledby="review-heading" className="space-y-4">
      <div className="flex items-center justify-between gap-3">
        <h2 id="review-heading" className="text-sm font-semibold uppercase tracking-wider text-muted-foreground">
          Review status
        </h2>
        <span className={`rounded-full border px-2.5 py-1 text-xs font-semibold ${overallStatus.className}`}>
          {overallStatus.label}
        </span>
      </div>

      {statusTypes.length > 0 ? (
        <div className="grid gap-2 sm:grid-cols-2">
          {statusTypes.map((type) => {
            const status = perTypeStatusPresentation(type.status)
            return (
              <div key={type.reviewType} className={`rounded-md border p-3 ${status.className}`}>
                <div className="flex items-center justify-between gap-2">
                  <span className="text-sm font-medium">{type.reviewType}</span>
                  <span className="text-xs font-semibold">{status.label}</span>
                </div>
                {type.outcome && (
                  <p className="mt-1 text-xs opacity-80">Outcome: {type.outcome}</p>
                )}
                {type.reviewerDisplayNames.length > 0 && (
                  <p className="mt-1 text-xs opacity-80">
                    Reviewed by {type.reviewerDisplayNames.join(", ")}
                  </p>
                )}
              </div>
            )
          })}
        </div>
      ) : (
        <p className="rounded-md border border-dashed p-3 text-sm text-muted-foreground">
          No configured review types
        </p>
      )}

      {reviewsLoading && (
        <p className="text-sm text-muted-foreground">Loading review findings...</p>
      )}
      {reviewsError && (
        <p role="alert" className="text-sm text-red-600 dark:text-red-400">
          Findings could not be loaded: {reviewsError}
        </p>
      )}
      {!reviewsLoading && !reviewsError && reviews.length === 0 && (
        <p className="rounded-md border border-dashed p-3 text-sm text-muted-foreground">
          No reviews have been submitted for this version.
        </p>
      )}
      {!reviewsLoading && !reviewsError && reviews.length > 0 && (
        <div className="space-y-3">
          <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            Findings
          </h3>
          {earlierReviews.length > 0 && (
            <Button
              type="button"
              variant="ghost"
              className="h-auto w-full justify-between px-2 py-1.5 text-sm text-muted-foreground"
              aria-expanded={showEarlierReviews}
              onClick={() => setShowEarlierReviews((expanded) => !expanded)}
            >
              <span>
                {earlierReviews.length} earlier review{earlierReviews.length === 1 ? "" : "s"}
              </span>
              {showEarlierReviews ? (
                <ChevronUp className="h-4 w-4" aria-hidden="true" />
              ) : (
                <ChevronDown className="h-4 w-4" aria-hidden="true" />
              )}
            </Button>
          )}
          {displayedReviews.map((review) => {
            const stale = review.is_stale === true
            return (
              <article
                key={review.id}
                data-testid={`review-finding-${review.id}`}
                className={`rounded-md border p-3 ${stale
                  ? "border-amber-500/40 bg-amber-500/5"
                  : "border-border bg-muted/20"}`}
              >
                <div className="flex flex-wrap items-center gap-2 text-xs">
                  <span className="font-semibold">{review.review_type}</span>
                  <span className="rounded-full border px-2 py-0.5">{review.outcome}</span>
                  {stale && (
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <span className="rounded-full border border-amber-500/50 px-2 py-0.5 text-amber-700 dark:text-amber-400">
                          Stale
                        </span>
                      </TooltipTrigger>
                      <TooltipContent>
                        <p>The artifact has changed since this review.</p>
                      </TooltipContent>
                    </Tooltip>
                  )}
                  {!stale && review.is_current === true && (
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <span className="rounded-full border border-green-500/50 px-2 py-0.5 text-green-700 dark:text-green-400">
                          Current
                        </span>
                      </TooltipTrigger>
                      <TooltipContent>
                        <p>The artifact hasn&apos;t changed since this review.</p>
                      </TooltipContent>
                    </Tooltip>
                  )}
                </div>
                <p className="mt-2 text-xs text-muted-foreground">
                  <span>{review.reviewer_display_name || "Unknown reviewer"}</span>
                  <span aria-hidden="true"> · </span>
                  <time dateTime={review.created_at}>{formatDate(review.created_at)}</time>
                </p>
                <p className="mt-2 whitespace-pre-wrap break-words text-sm">{review.notes}</p>
              </article>
            )
          })}
        </div>
      )}

      {showReview && (
        <div className="rounded-md border p-4">
          <h3 className="text-sm font-semibold">Submit a review</h3>
          {configError && (
            <p role="alert" className="mt-2 text-sm text-red-600 dark:text-red-400">
              Review options could not be loaded: {configError}
            </p>
          )}
          {!configError && frontendConfig && (
            <form onSubmit={handleSubmit} className="mt-3 space-y-3">
              <div className="grid gap-3 sm:grid-cols-2">
                <div className="space-y-1.5">
                  <Label htmlFor="review-type">Review type</Label>
                  <select
                    id="review-type"
                    value={reviewType}
                    onChange={(event) => setReviewType(event.target.value)}
                    disabled={submitting}
                    className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                  >
                    {frontendConfig.review_types.map((type) => (
                      <option key={type} value={type}>{type}</option>
                    ))}
                  </select>
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="review-outcome">Outcome</Label>
                  <select
                    id="review-outcome"
                    value={outcome}
                    onChange={(event) => setOutcome(event.target.value)}
                    disabled={submitting}
                    className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                  >
                    {frontendConfig.review_outcomes.map((value) => (
                      <option key={value} value={value}>{value}</option>
                    ))}
                  </select>
                </div>
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="review-notes">Findings</Label>
                <Textarea
                  id="review-notes"
                  value={notes}
                  onChange={(event) => setNotes(event.target.value)}
                  placeholder="Describe the review findings..."
                  disabled={submitting}
                  required
                />
              </div>
              {submissionError && (
                <p role="alert" className="text-sm text-red-600 dark:text-red-400">
                  {submissionError}
                </p>
              )}
              <Button type="submit" disabled={submitting}>
                {submitting ? "Submitting..." : "Submit review"}
              </Button>
            </form>
          )}
        </div>
      )}
      </section>
    </TooltipProvider>
  )
}
