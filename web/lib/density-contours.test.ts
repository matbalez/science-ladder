import { test } from "node:test";
import assert from "node:assert/strict";
import { densityContours } from "./density-contours.ts";

test("density interpolation preserves empty and full fields, without inventing material outside the mesh", () => {
  assert.ok(
    densityContours([
      [0, 0],
      [0, 0],
    ]).every((c) => c.polygons.length === 0),
  );
  const full = densityContours([
    [1, 1],
    [1, 1],
  ]);
  const area = (points: number[][]) =>
    Math.abs(
      points.reduce((a, p, i) => {
        const q = points[(i + 1) % points.length];
        return a + p[0] * q[1] - q[0] * p[1];
      }, 0),
    ) / 2;
  for (const c of full)
    assert.ok(
      Math.abs(c.polygons.reduce((sum, p) => sum + area(p), 0) - 4) < 1e-10,
    );
  for (const c of densityContours([
    [0, 1],
    [0.4, 0.8],
  ]))
    for (const p of c.polygons)
      for (const [x, y] of p) {
        assert.ok(
          Number.isFinite(x) &&
            Number.isFinite(y) &&
            x >= 0 &&
            x <= 2 &&
            y >= 0 &&
            y <= 2,
        );
      }
});
