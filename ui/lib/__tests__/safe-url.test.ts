import { describe, expect, it } from "vitest"
import { getSafeHttpUrl } from "../safe-url"

describe("getSafeHttpUrl", () => {
  it.each([
    ["https://example.com", "https://example.com"],
    ["http://example.com/path", "http://example.com/path"],
    ["javascript:alert(1)", null],
    ["data:text/html,<script>alert(1)</script>", null],
    ["vbscript:msgbox(1)", null],
    ["file:///tmp/example.txt", null],
    ["", null],
    ["   ", null],
    ["//evil.com/path", null],
    ["/relative/path", null],
  ])("returns %s as %s", (value, expected) => {
    expect(getSafeHttpUrl(value)).toBe(expected)
  })
})
