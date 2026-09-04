import test from "node:test";
import assert from "node:assert/strict";
import { formatTicks, plotRatio, safeWebUrl } from "./scientific.ts";
test("integer score display remains exact beyond JavaScript safe integers", () => {
  assert.equal(
    formatTicks("900719925474099312345", "0.000001"),
    "900,719,925,474,099.312345",
  );
  assert.equal(formatTicks("-100001", "0.001"), "-100.001");
  assert.equal(formatTicks("4", "0.25"), "1");
});
test("plot coordinates normalize huge nearby integers before converting", () => {
  assert.equal(
    plotRatio("9007199254740993", 9007199254740992n, 9007199254740994n),
    0.5,
  );
});
test("manifest links cannot execute javascript or data URLs", () => {
  assert.equal(safeWebUrl("javascript:alert(1)"), undefined);
  assert.equal(safeWebUrl("data:text/html,<script>"), undefined);
  assert.equal(
    safeWebUrl("https://arxiv.org/abs/1234.56789"),
    "https://arxiv.org/abs/1234.56789",
  );
});
