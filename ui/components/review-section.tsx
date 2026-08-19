"use client"

import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { ReviewTypeCard } from "@/components/review-type-card"
import { TooltipProvider } from "@/components/ui/tooltip"
import {
  createReviewV0,
  createReviewOverrideV0,
  getFrontendConfig,
  listReviewsV0,
  type CapabilitiesMeta,
  type FrontendConfigBody,
  type Review,
  type ReviewSummary,
} from "@/lib/admin-api"
import { capabilityFlags } from "@/lib/capabilities"
import { overallStatusPresentation, type StatusPresentation } from "@/lib/review-status"

interface ReviewSectionProps {
  artifactType: string
  artifactName: string
  artifactVersion: string
  reviewSummary?: ReviewSummary
  capabilities?: Partial<CapabilitiesMeta>
  onReviewSubmitted?: () => void | Promise<void>
}

type ReviewTypeCardEntry = {
  reviewType: string
  statusPresentation?: StatusPresentation
  reviews: Review[]
  unconfigured?: boolean
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
    case "overridden":
      return {
        label: "Overridden",
        className: "border-border bg-muted/20 text-muted-foreground",
      }
    default:
      return {
        label: "Pending review",
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
  const [overrideTargetID, setOverrideTargetID] = useState<number | undefined>()
  const [overrideReason, setOverrideReason] = useState("")
  const [overrideSubmitting, setOverrideSubmitting] = useState(false)
  const [overrideError, setOverrideError] = useState<string | null>(null)
  const [expandedReviewTypes, setExpandedReviewTypes] = useState<Record<string, boolean>>(
    () => Object.fromEntries(
      (reviewSummary?.per_type ?? []).map((type) => [type.review_type, type.status === "fail"]),
    ),
  )
  const reviewsRequest = useRef(0)

  const { showReview, showOverride } = capabilityFlags(capabilities)

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
    }))
  }, [reviewSummary])

  const reviewGroups = useMemo(() => {
    const groups = new Map<string, Review[]>()
    for (const review of reviews) {
      const group = groups.get(review.review_type) ?? []
      group.push(review)
      groups.set(review.review_type, group)
    }
    return groups
  }, [reviews])

  const overriddenReviewIDs = useMemo(
    () =>
      new Set(
        reviews.flatMap((review) =>
          review.overrides_review_id !== undefined ? [review.overrides_review_id] : [],
        ),
      ),
    [reviews],
  )

  const configuredReviewTypeNames = useMemo(
    () => new Set(statusTypes.map((type) => type.reviewType)),
    [statusTypes],
  )

  const unconfiguredReviewTypes = useMemo(
    () => Array.from(reviewGroups.keys()).filter((reviewType) => !configuredReviewTypeNames.has(reviewType)),
    [configuredReviewTypeNames, reviewGroups],
  )

  useEffect(() => {
    setExpandedReviewTypes((expanded) => {
      let next = expanded
      for (const type of statusTypes) {
        if (!Object.prototype.hasOwnProperty.call(expanded, type.reviewType)) {
          next = {
            ...next,
            [type.reviewType]: type.status === "fail",
          }
        }
      }
      return next
    })
  }, [statusTypes])

  const reviewTypeCards = useMemo<ReviewTypeCardEntry[]>(
    () => [
      ...statusTypes.map((type) => ({
        reviewType: type.reviewType,
        statusPresentation: perTypeStatusPresentation(type.status),
        reviews: reviewGroups.get(type.reviewType) ?? [],
      })),
      ...unconfiguredReviewTypes.map((reviewType) => ({
        reviewType,
        reviews: reviewGroups.get(reviewType) ?? [],
        unconfigured: true,
      })),
    ],
    [reviewGroups, statusTypes, unconfiguredReviewTypes],
  )

  const toggleReviewType = (reviewType: string) => {
    setExpandedReviewTypes((expanded) => ({
      ...expanded,
      [reviewType]: !expanded[reviewType],
    }))
  }

  const startOverride = (review: Review) => {
    setOverrideTargetID(review.id)
    setOverrideReason("")
    setOverrideError(null)
  }

  const cancelOverride = () => {
    if (overrideSubmitting) {
      return
    }
    setOverrideTargetID(undefined)
    setOverrideReason("")
    setOverrideError(null)
  }

  const handleOverride = async (review: Review) => {
    const reason = overrideReason.trim()
    if (!reason) {
      setOverrideError("Override reason is required")
      return
    }

    setOverrideSubmitting(true)
    setOverrideError(null)
    try {
      await createReviewOverrideV0({
        path: {
          artifactType,
          artifactName,
          version: artifactVersion,
        },
        body: {
          review_id: review.id,
          reason,
        },
        throwOnError: true,
      })
      toast.success("Review override recorded")
      setOverrideTargetID(undefined)
      setOverrideReason("")
      await loadReviews()
      await onReviewSubmitted?.()
    } catch (error) {
      const message = extractErrorMessage(error, "Unable to record review override")
      setOverrideError(message)
      toast.error(message)
    } finally {
      setOverrideSubmitting(false)
    }
  }

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

  const overallStatus = overallStatusPresentation(reviewSummary?.status ?? "pending", "detail")
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

      {reviewTypeCards.length > 0 ? (
        <div className="space-y-3">
          {reviewTypeCards.map((type) => (
            <ReviewTypeCard
              key={type.reviewType}
              reviewType={type.reviewType}
              statusPresentation={type.statusPresentation}
              reviews={type.reviews}
              unconfigured={type.unconfigured}
              expanded={expandedReviewTypes[type.reviewType] === true}
              onToggle={() => toggleReviewType(type.reviewType)}
              canOverride={showOverride}
              failureOutcome={frontendConfig?.review_failure_outcome}
              overriddenReviewIDs={overriddenReviewIDs}
              overrideTargetID={overrideTargetID}
              overrideReason={overrideReason}
              overrideError={overrideError ?? undefined}
              overrideSubmitting={overrideSubmitting}
              onStartOverride={startOverride}
              onOverrideReasonChange={setOverrideReason}
              onCancelOverride={cancelOverride}
              onSubmitOverride={handleOverride}
            />
          ))}
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
