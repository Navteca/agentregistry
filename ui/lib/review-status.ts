export type StatusPresentation = {
  label: string
  className: string
}

export type StatusPresentationVariant = "detail" | "card"

export function overallStatusPresentation(
  status?: string,
  variant: StatusPresentationVariant = "detail",
): StatusPresentation {
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
        label: variant === "card" ? "Pending Review" : "Pending",
        className: "border-amber-500/30 bg-amber-500/5 text-amber-700 dark:text-amber-400",
      }
  }
}
