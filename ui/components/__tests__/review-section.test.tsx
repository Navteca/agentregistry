import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { ReviewSection } from "../review-section"
import { createReviewV0, getFrontendConfig, listReviewsV0 } from "@/lib/admin-api"

vi.mock("@/lib/admin-api", () => ({
  createReviewV0: vi.fn(),
  getFrontendConfig: vi.fn(),
  listReviewsV0: vi.fn(),
}))

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

const frontendConfig = {
  keycloak_url: "",
  keycloak_realm: "",
  keycloak_client_id: "",
  anonymous_auth_enabled: true,
  review_types: ["security", "scientific"],
  review_outcomes: ["pass", "fail"],
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
    vi.mocked(getFrontendConfig).mockResolvedValue({ data: frontendConfig } as never)
    vi.mocked(listReviewsV0).mockResolvedValue({ data: [] } as never)
    vi.mocked(createReviewV0).mockResolvedValue({ data: review() } as never)
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
    expect(screen.getAllByText("Pending").length).toBeGreaterThan(0)
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
    expect(screen.getAllByText("Pending").length).toBeGreaterThan(0)
    expect(screen.queryByText("Rejected")).not.toBeInTheDocument()
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

  it("renders a placeholder when an artifact has no reviews", async () => {
    render(<ReviewSection {...baseProps} reviewSummary={reviewSummary("pending")} />)

    expect(await screen.findByText("No reviews have been submitted for this version.")).toBeInTheDocument()
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
    vi.mocked(getFrontendConfig).mockResolvedValue({
      data: {
        ...frontendConfig,
        review_types: ["legal"],
        review_outcomes: ["approve", "reject", "needs-work"],
      },
    } as never)
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

  it("renders reviewer, date, outcome, and notes for findings", async () => {
    vi.mocked(listReviewsV0).mockResolvedValue({ data: [review()] } as never)
    const user = userEvent.setup()
    render(<ReviewSection {...baseProps} />)

    expect(await screen.findByText("Bob Curator")).toBeInTheDocument()
    expect(screen.getByText(/January 2, 2026/)).toBeInTheDocument()
    expect(screen.getByText("pass")).toBeInTheDocument()
    expect(screen.getByText("Looks good")).toBeInTheDocument()
    await user.hover(screen.getByText("Current"))
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "The artifact hasn't changed since this review.",
    )
  })

  it("visually distinguishes stale findings", async () => {
    vi.mocked(listReviewsV0).mockResolvedValue({
      data: [review({ is_current: false, is_stale: true })],
    } as never)
    const user = userEvent.setup()
    render(<ReviewSection {...baseProps} />)

    expect(await screen.findByText("Stale")).toBeInTheDocument()
    expect(screen.getByTestId("review-finding-1")).toHaveClass("border-amber-500/40")
    await user.hover(screen.getByText("Stale"))
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "The artifact has changed since this review.",
    )
  })

  it("renders finding markup as escaped text", async () => {
    const notes = '<script>alert("x")</script>'
    vi.mocked(listReviewsV0).mockResolvedValue({ data: [review({ notes })] } as never)
    const { container } = render(<ReviewSection {...baseProps} />)

    const finding = await screen.findByTestId("review-finding-1")
    expect(finding).toHaveTextContent(notes)
    expect(container.querySelector("script")).not.toBeInTheDocument()
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
