import { test, expect } from "@playwright/test";

for (const host of ["science-ladder.fly.dev", "www.scienceladder.org"]) {
  test(`${host} preserves page paths and query strings on the canonical domain`, async ({
    request,
  }) => {
    for (const path of [
      "/",
      "/challenges/one-less-multiply",
      "/review?version=test-version",
      "/authorize?user_code=TEST-CODE",
    ]) {
      const response = await request.get(path, {
        headers: { host },
        maxRedirects: 0,
      });
      expect(response.status()).toBe(308);
      expect(response.headers().location).toBe(
        `https://scienceladder.org${path}`,
      );
    }
    const auth = await request.get("/v1/auth/github", {
      headers: { host },
      maxRedirects: 0,
    });
    expect(auth.status()).toBe(307);
    expect(auth.headers().location).toBe(
      "https://scienceladder.org/v1/auth/github",
    );
  });
  test(`${host} does not redirect legacy API, key discovery, or assets`, async ({
    request,
  }) => {
    for (const path of [
      "/v1/me",
      "/v1/auth/github/callback",
      "/.well-known/science-ladder-keys.json",
      "/_next/static/missing.js",
    ]) {
      const response = await request.get(path, {
        headers: { host },
        maxRedirects: 0,
      });
      expect(response.headers().location).toBeUndefined();
    }
    const mutation = await request.post("/v1/submission-intents", {
      headers: { host, Authorization: "Bearer TEST-ONLY" },
      data: {},
      maxRedirects: 0,
    });
    expect(mutation.headers().location).toBeUndefined();
  });
}
test("canonical and local hosts do not loop", async ({ request }) => {
  for (const host of ["scienceladder.org", "localhost:3018"]) {
    const response = await request.get("/", {
      headers: { host },
      maxRedirects: 0,
    });
    expect(response.status()).toBe(200);
    expect(response.headers().location).toBeUndefined();
  }
});
