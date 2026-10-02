import { test, expect, Page } from "@playwright/test";
import { randomUUID } from "node:crypto";

test.use({ baseURL: "http://127.0.0.1:4783" });

async function login(page: Page, user: string, password: string) {
  await page.goto("http://127.0.0.1:4783/");
  await page.getByLabel("Account name", { exact: true }).fill(user);
  await page.getByLabel("Password", { exact: true }).fill(password);
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Create a world", exact: true }),
  ).toBeVisible();
}

test("hosted reader changes password and revokes other browser sessions", async ({
  page,
  browser,
}) => {
  const user = "author-" + randomUUID().slice(0, 8);
  await login(page, "admin", "browser-test-admin-password");
  const world = await page.evaluate(async (user) => {
    async function post(path: string, body: unknown) {
      const r = await fetch("/api/v1" + path, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      if (!r.ok) throw Error(await r.text());
      return r.json();
    }
    await post("/accounts", { user, password: "browser-original-password" });
    const s = await post("/projects", {
      name: "Shared test " + user,
      age: "First",
    });
    await post(`/projects/${s.root.world.id}/members`, {
      user,
      role: "viewer",
    });
    return s.root.world.id as string;
  }, user);
  const first = await browser.newContext(),
    second = await browser.newContext();
  try {
    const author = await first.newPage(),
      other = await second.newPage();
    await login(author, user, "browser-original-password");
    await login(other, user, "browser-original-password");
    await author.goto(`http://127.0.0.1:4783/?world=${world}&view=access`);
    await expect(
      author.getByText("Your world role is viewer.", { exact: false }),
    ).toBeVisible();
    const forbidden = await author.evaluate(
      async (world) =>
        (
          await fetch(`/api/v1/projects/${world}/commands`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: "{}",
          })
        ).status,
      world,
    );
    expect(forbidden).toBe(403);
    await author.getByText("Password and sessions", { exact: true }).click();
    await author
      .getByLabel("Your current password", { exact: true })
      .fill("browser-original-password");
    await author
      .getByLabel("New password", { exact: true })
      .fill("browser-replacement-password");
    await author
      .getByLabel("Confirm new password", { exact: true })
      .fill("browser-replacement-password");
    await author
      .getByRole("button", {
        name: "Change password and sign out",
        exact: true,
      })
      .click();
    await expect(
      author.getByRole("button", { name: "Sign in", exact: true }),
    ).toBeVisible();
    await other.reload();
    await expect(
      other.getByRole("button", { name: "Sign in", exact: true }),
    ).toBeVisible();
    await login(author, user, "browser-replacement-password");
    await author.goto(`http://127.0.0.1:4783/?world=${world}`);
    await expect(
      author.getByText("Read-only world access.", { exact: false }),
    ).toBeVisible();
  } finally {
    await first.close();
    await second.close();
  }
});
