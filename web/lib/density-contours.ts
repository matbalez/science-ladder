export type Point = [number, number];
type Sample = { x: number; y: number; value: number };

// Piecewise-linear contours of a bilinearly interpolated cell-centred field.
// This is a display interpolation, never an input to the finite-element solver.
export function densityContours(
  grid: number[][],
): { level: number; polygons: Point[][] }[] {
  const ny = grid.length,
    nx = grid[0]?.length || 0;
  if (!nx || !ny) return [];
  const sample = (x: number, y: number): Sample => {
    const gx = Math.max(0, Math.min(nx - 1, x - 0.5));
    const gy = Math.max(0, Math.min(ny - 1, y - 0.5));
    const x0 = Math.floor(gx),
      y0 = Math.floor(gy);
    const x1 = Math.min(x0 + 1, nx - 1),
      y1 = Math.min(y0 + 1, ny - 1);
    const tx = gx - x0,
      ty = gy - y0;
    return {
      x,
      y,
      value:
        (1 - ty) * ((1 - tx) * grid[y0][x0] + tx * grid[y0][x1]) +
        ty * ((1 - tx) * grid[y1][x0] + tx * grid[y1][x1]),
    };
  };
  const triangles: Sample[][] = [];
  for (let y = 0; y < ny; y += 0.5)
    for (let x = 0; x < nx; x += 0.5) {
      const a = sample(x, y),
        b = sample(x + 0.5, y),
        c = sample(x + 0.5, y + 0.5),
        d = sample(x, y + 0.5);
      triangles.push([a, b, c], [a, c, d]);
    }
  return [0.1, 0.3, 0.5, 0.7, 0.9].map((level) => {
    const polygons: Point[][] = [];
    for (const triangle of triangles) {
      const vertices: Point[] = [];
      for (let j = 0; j < 3; j++) {
        const a = triangle[j],
          b = triangle[(j + 1) % 3];
        if (a.value >= level) vertices.push([a.x, a.y]);
        if (a.value >= level !== b.value >= level) {
          const t = (level - a.value) / (b.value - a.value);
          vertices.push([a.x + t * (b.x - a.x), a.y + t * (b.y - a.y)]);
        }
      }
      if (vertices.length >= 3) polygons.push(vertices);
    }
    return { level, polygons };
  });
}

export function densityColor(density: number) {
  return `rgb(${Math.round(235 - density * 199)},${Math.round(229 - density * 190)},${Math.round(216 - density * 181)})`;
}
