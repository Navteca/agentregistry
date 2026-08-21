import { render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { ReviewSection as ReviewSectionComponent } from "../review-section"
import { createReviewOverrideV0, createReviewV0, listReviewsV0 } from "@/lib/admin-api"
import { FrontendConfigProvider, type FrontendConfig } from "@/lib/frontend-config"

vi.mock("@/lib/admin-api", () => ({
  createReviewV0: vi.fn(),
  createReviewOverrideV0: vi.fn(),
  listReviewsV0: vi.fn(),
}))

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

const frontendConfig: FrontendConfig = {
  keycloak_url: "",
  keycloak_realm: "",
  keycloak_client_id: "",
  anonymous_auth_enabled: true,
  show_github_link: true,
  show_discord_link: true,
  review_types: ["security", "scientific"],
  review_outcomes: ["pass", "fail"],
  review_failure_outcome: "fail",
  review_override_outcome: "override",
}

let currentFrontendConfig = frontendConfig

type ReviewSectionProps = Parameters<typeof ReviewSectionComponent>[0]

function ReviewSection(props: ReviewSectionProps) {
  return (
    <FrontendConfigProvider config={currentFrontendConfig}>
      <ReviewSectionComponent {...props} />
    </FrontendConfigProvider>
  )
}

const baseProps = {
  artifactType: "server",
  artifactName: "io.example/server",
  artifactVersion: "1.0.0",
}

const reviewSummary = (status: string, perType = []) => ({
  status,
  per_type: perType,
})

const review = (overrides = {}) => ({
  id: 1,
  artifact_name: "io.example/server",
  artifact_type: "server",
  artifact_version: "1.0.0",
  created_at: "2026-01-02T15:04:05Z",
  notes: "Looks good",
  outcome: "pass",
  review_type: "security",
  reviewer_auth_method: "oidc",
  reviewer_display_name: "Bob Curator",
  reviewer_subject: "bob",
  is_current: true,
  is_stale: false,
  ...overrides,
})

describe("ReviewSection", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    currentFrontendConfig = frontendConfig
    vi.mocked(listReviewsV0).mockResolvedValue({ data: [] } as never)
    vi.mocked(createReviewV0).mockResolvedValue({ data: review() } as never)
    vi.mocked(createReviewOverrideV0).mockResolvedValue({ data: review({ overrides_review_id: 1, outcome: "override" }) } as never)
  })

  it.each([
    ["certified", "Certified"],
    ["rejected", "Rejected"],
    ["pending", "Pending"],
  ])("renders %s status distinctly", (status, label) => {
    render(
      <ReviewSection
        {...baseProps}
        reviewSummary={reviewSummary(status)}
      />,
    )

    expect(screen.getByText(label)).toBeInTheDocument()
  })

  it("renders a configured type with no review as pending", async () => {
    render(
      <ReviewSection
        {...baseProps}
        reviewSummary={reviewSummary("pending", [
          { review_type: "security", status: "pass" },
          { review_type: "scientific", status: "pending" },
        ])}
      />,
    )

    expect(await screen.findByText("scientific")).toBeInTheDocument()
    expect(screen.getByText("Pending review")).toBeInTheDocument()
    expect(screen.getByText("Passed")).toBeInTheDocument()
    expect(screen.queryByText("Certified")).not.toBeInTheDocument()
  })

  it("renders per-type failures as failed without changing the overall status vocabulary", () => {
    render(
      <ReviewSection
        {...baseProps}
        reviewSummary={reviewSummary("pending", [
          { review_type: "security", status: "fail" },
          { review_type: "scientific", status: "pending" },
        ])}
      />,
    )

    expect(screen.getByText("Failed")).toBeInTheDocument()
    expect(screen.getByText("Pending review")).toBeInTheDocument()
    expect(screen.queryByText("Rejected")).not.toBeInTheDocument()
  })

  it("renders overridden as a distinct per-type status", () => {
    render(
      <ReviewSection
        {...baseProps}
        reviewSummary={reviewSummary("certified", [{ review_type: "security", status: "overridden" }])}
      />,
    )

    expect(screen.getByText("Overridden")).toBeInTheDocument()
    expect(screen.getByText("Certified")).toBeInTheDocument()
    expect(screen.queryByText("Passed")).not.toBeInTheDocument()
  })

  it("shows the override control only to callers with the override capability", async () => {
    vi.mocked(listReviewsV0).mockResolvedValue({
      data: [review({ outcome: "fail", notes: "blocking finding" })],
    } as never)
    const { rerender } = render(
      <ReviewSection
        {...baseProps}
        capabilities={{ can_override: true }}
        reviewSummary={reviewSummary("rejected", [{ review_type: "security", status: "fail" }])}
      />,
    )

    expect(await screen.findByRole("button", { name: "Override failed review" })).toBeInTheDocument()

    rerender(
      <ReviewSection
        {...baseProps}
        capabilities={{ can_review: true }}
        reviewSummary={reviewSummary("rejected", [{ review_type: "security", status: "fail" }])}
      />,
    )
    expect(screen.queryByRole("button", { name: "Override failed review" })).not.toBeInTheDocument()
  })

  it("records an override with the selected failure and required reason", async () => {
    vi.mocked(listReviewsV0).mockResolvedValue({
      data: [review({ outcome: "fail", notes: "blocking finding" })],
    } as never)
    const user = userEvent.setup()
    render(
      <ReviewSection
        {...baseProps}
        capabilities={{ can_override: true }}
        reviewSummary={reviewSummary("rejected", [{ review_type: "security", status: "fail" }])}
      />,
    )

    await user.click(await screen.findByRole("button", { name: "Override failed review" }))
    await user.type(screen.getByLabelText("Override reason"), "Accepted risk after compensating control")
    await user.click(screen.getByRole("button", { name: "Record override" }))

    await waitFor(() =>
      expect(createReviewOverrideV0).toHaveBeenCalledWith({
        path: {
          artifactType: "server",
          artifactName: "io.example/server",
          version: "1.0.0",
        },
        body: {
          review_id: 1,
          reason: "Accepted risk after compensating control",
        },
        throwOnError: true,
      }),
    )
  })

  it("shows the failed review and its override in the same type card", async () => {
    vi.mocked(listReviewsV0).mockResolvedValue({
      data: [
        review({ id: 1, outcome: "fail", notes: "blocking finding" }),
        review({
          id: 2,
          outcome: "override",
          notes: "Accepted risk",
          reviewer_display_name: "Alice Admin",
          overrides_review_id: 1,
        }),
      ],
    } as never)
    const user = userEvent.setup()
    render(
      <ReviewSection
        {...baseProps}
        reviewSummary={reviewSummary("certified", [{ review_type: "security", status: "overridden" }])}
      />,
    )

    await user.click(await screen.findByRole("button", { name: "2 reviews for security" }))
    expect(screen.getByText("blocking finding")).toBeInTheDocument()
    expect(screen.getByText("Accepted risk")).toBeInTheDocument()
    expect(screen.getByText("Alice Admin")).toBeInTheDocument()
    expect(screen.getByText("Overrides review #1")).toBeInTheDocument()
    expect(screen.queryByRole("button", { name: "Override failed review" })).not.toBeInTheDocument()
  })

  it("uses the artifact summary as the authoritative status type list", () => {
    render(
      <ReviewSection
        {...baseProps}
        reviewSummary={reviewSummary("pending", [
          { review_type: "backend-only", status: "pending" },
        ])}
      />,
    )

    expect(screen.getByText("backend-only")).toBeInTheDocument()
    expect(screen.queryByText("security")).not.toBeInTheDocument()
  })

  it("shows per-type empty states without a duplicate section-level message", () => {
    render(
      <ReviewSection
        {...baseProps}
        reviewSummary={reviewSummary("pending", [
          { review_type: "security", status: "pending" },
          { review_type: "scientific", status: "pending" },
        ])}
      />,
    )

    expect(screen.getAllByText("No reviews yet")).toHaveLength(2)
    expect(screen.queryByText("No reviews have been submitted for this version.")).not.toBeInTheDocument()
  })

  it("shows the form only when can_review is true", async () => {
    render(
      <ReviewSection
        {...baseProps}
        capabilities={{ can_review: true }}
      />,
    )

    expect(await screen.findByRole("button", { name: "Submit review" })).toBeInTheDocument()
  })

  it("populates form inputs from the configured vocabulary", async () => {
    currentFrontendConfig = {
      ...frontendConfig,
      review_types: ["legal"],
      review_outcomes: ["approve", "reject", "needs-work"],
    }
    render(
      <ReviewSection
        {...baseProps}
        capabilities={{ can_review: true }}
      />,
    )

    expect(await screen.findByLabelText("Review type")).toHaveValue("legal")
    expect(screen.getByRole("option", { name: "approve" })).toBeInTheDocument()
    expect(screen.getByRole("option", { name: "needs-work" })).toBeInTheDocument()
    expect(screen.queryByRole("option", { name: "pass" })).not.toBeInTheDocument()
  })

  it("hides the form when can_review is false or capabilities are absent", async () => {
    const { rerender } = render(
      <ReviewSection
        {...baseProps}
        capabilities={{ can_review: false }}
      />,
    )
    await waitFor(() => expect(screen.queryByRole("button", { name: "Submit review" })).not.toBeInTheDocument())

    rerender(<ReviewSection {...baseProps} />)
    await waitFor(() => expect(screen.queryByRole("button", { name: "Submit review" })).not.toBeInTheDocument())
  })

  it("renders one card per configured type with its status", () => {
    render(
      <ReviewSection
        {...baseProps}
        reviewSummary={reviewSummary("pending", [
          { review_type: "security", status: "pass" },
          { review_type: "scientific", status: "fail" },
        ])}
      />,
    )

    expect(screen.getByTestId("review-type-card-security")).toBeInTheDocument()
    expect(screen.getByTestId("review-type-card-scientific")).toBeInTheDocument()
    expect(screen.getByText("Passed")).toBeInTheDocument()
    expect(screen.getByText("Failed")).toBeInTheDocument()
  })

  it("sanitizes review type names in card test IDs", () => {
    render(
      <ReviewSection
        {...baseProps}
        reviewSummary={reviewSummary("pending", [{ review_type: "security.review", status: "pending" }])}
      />,
    )

    expect(screen.getByTestId("review-type-card-security-review")).toBeInTheDocument()
  })

  it("applies failing expansion when the summary arrives after mount", async () => {
    vi.mocked(listReviewsV0).mockResolvedValue({
      data: [review({ notes: "late failing finding" })],
    } as never)
    const { rerender } = render(<ReviewSection {...baseProps} />)

    rerender(
      <ReviewSection
        {...baseProps}
        reviewSummary={reviewSummary("rejected", [{ review_type: "security", status: "fail" }])}
      />,
    )

    expect(await screen.findByText("late failing finding")).toBeInTheDocument()
  })

  it("preserves a manual collapse across a summary refresh", async () => {
    vi.mocked(listReviewsV0).mockResolvedValue({
      data: [review({ notes: "collapsible failing finding" })],
    } as never)
    const summary = reviewSummary("rejected", [{ review_type: "security", status: "fail" }])
    const user = userEvent.setup()
    const { rerender } = render(<ReviewSection {...baseProps} reviewSummary={summary} />)

    expect(await screen.findByText("collapsible failing finding")).toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: "1 review for security" }))
    expect(screen.queryByText("collapsible failing finding")).not.toBeInTheDocument()

    rerender(
      <ReviewSection
        {...baseProps}
        reviewSummary={reviewSummary("rejected", [{ review_type: "security", status: "fail" }])}
      />,
    )
    expect(screen.queryByText("collapsible failing finding")).not.toBeInTheDocument()
  })

  it("starts failing types expanded and passing and pending types collapsed", async () => {
    vi.mocked(listReviewsV0).mockResolvedValue({
      data: [
        review({ id: 1, review_type: "security", notes: "security finding", outcome: "pass" }),
        review({ id: 2, review_type: "scientific", notes: "scientific finding", outcome: "fail" }),
        review({ id: 3, review_type: "legal", notes: "legal finding", outcome: "pass" }),
      ],
    } as never)
    render(
      <ReviewSection
        {...baseProps}
        reviewSummary={reviewSummary("rejected", [
          { review_type: "security", status: "pass" },
          { review_type: "scientific", status: "fail" },
          { review_type: "legal", status: "pending" },
        ])}
      />,
    )

    expect(await screen.findByText("scientific finding")).toBeInTheDocument()
    expect(screen.queryByText("security finding")).not.toBeInTheDocument()
    expect(screen.queryByText("legal finding")).not.toBeInTheDocument()
  })

  it("shows the review count in each type toggle", async () => {
    vi.mocked(listReviewsV0).mockResolvedValue({
      data: [
        review({ id: 1, notes: "first security finding" }),
        review({ id: 2, notes: "second security finding" }),
      ],
    } as never)
    render(
      <ReviewSection
        {...baseProps}
        reviewSummary={reviewSummary("pending", [{ review_type: "security", status: "pass" }])}
      />,
    )

    expect(await screen.findByRole("button", { name: "2 reviews for security" })).toBeInTheDocument()
  })

  it("does not render a toggle or empty expansion for a type with no reviews", () => {
    render(
      <ReviewSection
        {...baseProps}
        reviewSummary={reviewSummary("pending", [
          { review_type: "security", status: "pass" },
          { review_type: "scientific", status: "pending" },
        ])}
      />,
    )

    expect(screen.getAllByText("No reviews yet")).toHaveLength(2)
    expect(screen.queryByRole("button", { name: /reviews for/ })).not.toBeInTheDocument()
    expect(screen.queryByTestId(/review-finding-/)).not.toBeInTheDocument()
  })

  it("expands and collapses only the selected type card", async () => {
    vi.mocked(listReviewsV0).mockResolvedValue({
      data: [
        review({ id: 1, review_type: "security", notes: "security finding" }),
        review({ id: 2, review_type: "scientific", notes: "scientific finding" }),
      ],
    } as never)
    const user = userEvent.setup()
    render(
      <ReviewSection
        {...baseProps}
        reviewSummary={reviewSummary("pending", [
          { review_type: "security", status: "pass" },
          { review_type: "scientific", status: "pass" },
        ])}
      />,
    )

    expect(await screen.findByRole("button", { name: "1 review for security" })).toBeInTheDocument()
    expect(screen.queryByText("security finding")).not.toBeInTheDocument()
    expect(screen.queryByText("scientific finding")).not.toBeInTheDocument()

    await user.click(screen.getByRole("button", { name: "1 review for security" }))
    expect(screen.getByText("security finding")).toBeInTheDocument()
    expect(screen.queryByText("scientific finding")).not.toBeInTheDocument()

    await user.click(screen.getByRole("button", { name: "1 review for security" }))
    expect(screen.queryByText("security finding")).not.toBeInTheDocument()
  })

  it("shows two current reviews of the same type in one card", async () => {
    vi.mocked(listReviewsV0).mockResolvedValue({
      data: [
        review({ id: 1, notes: "newer finding", created_at: "2026-01-03T15:04:05Z" }),
        review({ id: 2, notes: "older finding", created_at: "2026-01-02T15:04:05Z" }),
      ],
    } as never)
    const user = userEvent.setup()
    render(
      <ReviewSection
        {...baseProps}
        reviewSummary={reviewSummary("pending", [{ review_type: "security", status: "pass" }])}
      />,
    )

    await user.click(await screen.findByRole("button", { name: "2 reviews for security" }))
    const card = screen.getByTestId("review-type-card-security")
    expect(within(card).getByText("newer finding")).toBeInTheDocument()
    expect(within(card).getByText("older finding")).toBeInTheDocument()
    expect(card.textContent!.indexOf("newer finding")).toBeLessThan(card.textContent!.indexOf("older finding"))
  })

  it.each([
    ["Current", "The artifact hasn't changed since this review.", { is_current: true }],
    ["Superseded", "This reviewer has since submitted a newer review.", { is_current: false, is_superseded: true }],
    ["Stale", "The artifact has changed since this review.", { is_current: false, is_stale: true }],
  ])("renders the %s state badge with a help tooltip", async (label, tooltip, flags) => {
    vi.mocked(listReviewsV0).mockResolvedValue({ data: [review(flags)] } as never)
    const user = userEvent.setup()
    render(
      <ReviewSection
        {...baseProps}
        reviewSummary={reviewSummary("pending", [{ review_type: "security", status: "fail" }])}
      />,
    )

    const badge = await screen.findByText(label)
    const helpIcon = badge.querySelector("svg")
    expect(helpIcon).toBeInTheDocument()
    await user.hover(helpIcon!)
    expect(await screen.findByRole("tooltip")).toHaveTextContent(tooltip)
  })

  it("shows superseded and stale badges together when both facts are true", async () => {
    vi.mocked(listReviewsV0).mockResolvedValue({
      data: [review({ is_current: false, is_superseded: true, is_stale: true })],
    } as never)
    render(
      <ReviewSection
        {...baseProps}
        reviewSummary={reviewSummary("pending", [{ review_type: "security", status: "fail" }])}
      />,
    )

    expect(await screen.findByText("Superseded")).toBeInTheDocument()
    expect(screen.getByText("Stale")).toBeInTheDocument()
  })

  it("renders reviewer, date, outcome, and notes for findings", async () => {
    vi.mocked(listReviewsV0).mockResolvedValue({ data: [review()] } as never)
    const user = userEvent.setup()
    render(
      <ReviewSection
        {...baseProps}
        reviewSummary={reviewSummary("pending", [{ review_type: "security", status: "fail" }])}
      />,
    )

    expect(await screen.findByText("Bob Curator")).toBeInTheDocument()
    expect(screen.getByText(/January 2, 2026/)).toBeInTheDocument()
    expect(screen.getByText("pass")).toBeInTheDocument()
    expect(screen.getByText("Looks good")).toBeInTheDocument()
    await user.hover(screen.getByText("Current"))
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "The artifact hasn't changed since this review.",
    )
  })

  it("renders stale findings with the existing row styling", async () => {
    vi.mocked(listReviewsV0).mockResolvedValue({
      data: [review({ is_current: false, is_stale: true })],
    } as never)
    render(
      <ReviewSection
        {...baseProps}
        reviewSummary={reviewSummary("pending", [{ review_type: "security", status: "fail" }])}
      />,
    )

    expect(await screen.findByText("Stale")).toBeInTheDocument()
    expect(screen.getByTestId("review-finding-1")).toHaveClass("border-border", "bg-muted/20")
    expect(screen.getByTestId("review-finding-1")).not.toHaveClass("border-amber-500/40")
  })

  it("does not show another type's findings in a configured card", async () => {
    vi.mocked(listReviewsV0).mockResolvedValue({
      data: [
        review({ id: 1, review_type: "security", notes: "security finding" }),
        review({ id: 2, review_type: "scientific", notes: "scientific finding" }),
      ],
    } as never)
    const user = userEvent.setup()
    render(
      <ReviewSection
        {...baseProps}
        reviewSummary={reviewSummary("rejected", [
          { review_type: "security", status: "fail" },
          { review_type: "scientific", status: "pass" },
        ])}
      />,
    )

    const securityCard = screen.getByTestId("review-type-card-security")
    const scientificCard = screen.getByTestId("review-type-card-scientific")
    expect(await within(securityCard).findByText("security finding")).toBeInTheDocument()
    expect(within(securityCard).queryByText("scientific finding")).not.toBeInTheDocument()
    await user.click(within(scientificCard).getByRole("button", { name: "1 review for scientific" }))
    expect(within(scientificCard).getByText("scientific finding")).toBeInTheDocument()
  })

  it("renders finding markup as escaped text", async () => {
    const notes = '<script>alert("x")</script>'
    vi.mocked(listReviewsV0).mockResolvedValue({ data: [review({ notes })] } as never)
    const { container } = render(
      <ReviewSection
        {...baseProps}
        reviewSummary={reviewSummary("pending", [{ review_type: "security", status: "fail" }])}
      />,
    )

    const finding = await screen.findByTestId("review-finding-1")
    expect(finding).toHaveTextContent(notes)
    expect(container.querySelector("script")).not.toBeInTheDocument()
  })

  it("renders unconfigured findings in a trailing collapsed card", async () => {
    vi.mocked(listReviewsV0).mockResolvedValue({
      data: [
        review({ id: 1, notes: "configured finding" }),
        review({ id: 2, review_type: "legacy", notes: "legacy finding", outcome: "fail" }),
      ],
    } as never)
    const user = userEvent.setup()
    render(
      <ReviewSection
        {...baseProps}
        reviewSummary={reviewSummary("rejected", [{ review_type: "security", status: "fail" }])}
      />,
    )

    const configuredCard = screen.getByTestId("review-type-card-security")
    const legacyCard = await screen.findByTestId("review-type-card-legacy")
    expect(within(legacyCard).getByText("No longer configured")).toBeInTheDocument()
    expect(within(legacyCard).queryByText("legacy finding")).not.toBeInTheDocument()
    expect(configuredCard.compareDocumentPosition(legacyCard) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()

    await user.click(within(legacyCard).getByRole("button", { name: "1 review for legacy" }))
    expect(within(legacyCard).getByText("legacy finding")).toBeInTheDocument()
  })

  it("surfaces submission failures", async () => {
    vi.mocked(createReviewV0).mockRejectedValue(new Error("review permission denied"))
    const user = userEvent.setup()
    render(
      <ReviewSection
        {...baseProps}
        capabilities={{ can_review: true }}
      />,
    )

    await user.type(await screen.findByLabelText("Findings"), "Unable to certify")
    await user.click(screen.getByRole("button", { name: "Submit review" }))

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("review permission denied")
    })
  })

  it("keeps the summary rendered when findings fail to load", async () => {
    vi.mocked(listReviewsV0).mockRejectedValue(new Error("findings unavailable"))
    render(
      <ReviewSection
        {...baseProps}
        reviewSummary={reviewSummary("certified")}
      />,
    )

    expect(screen.getByText("Certified")).toBeInTheDocument()
    expect(await screen.findByText(/Findings could not be loaded: findings unavailable/)).toBeInTheDocument()
  })
})
