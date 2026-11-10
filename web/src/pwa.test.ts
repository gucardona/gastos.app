import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const root = join(__dirname, "..");
const html = readFileSync(join(root, "index.html"), "utf8");
const pub = (p: string) => join(root, "public", p.replace(/^\//, ""));

describe("installable app basics", () => {
  it("the page links a manifest, an SVG favicon, an apple-touch-icon and a theme color per scheme", () => {
    expect(html).toContain('<link rel="manifest" href="/manifest.webmanifest" />');
    expect(html).toMatch(/<link rel="icon" href="\/icon\.svg" type="image\/svg\+xml" \/>/);
    expect(html).toMatch(/<link rel="apple-touch-icon" href="\/icon-192\.png" \/>/);
    expect(html).toMatch(/<meta name="theme-color" content="#F2F3F4" media="\(prefers-color-scheme: light\)" \/>/);
    expect(html).toMatch(/<meta name="theme-color" content="#0F1113" media="\(prefers-color-scheme: dark\)" \/>/);
  });

  it("has no inline script (the server's CSP allows only script-src 'self')", () => {
    expect([...html.matchAll(/<script(?![^>]*\bsrc=)[^>]*>/g)]).toEqual([]);
  });

  it("the manifest describes the app with the token colors, and every icon it names exists", () => {
    const m = JSON.parse(readFileSync(pub("manifest.webmanifest"), "utf8"));
    expect(m).toMatchObject({ name: "gastos", short_name: "gastos", start_url: "/", display: "standalone", background_color: "#F2F3F4", theme_color: "#F2F3F4" });
    expect(m.icons.map((i: { sizes: string }) => i.sizes)).toEqual(expect.arrayContaining(["any", "192x192", "512x512"]));
    for (const icon of m.icons) expect(existsSync(pub(icon.src)), icon.src).toBe(true);
  });

  it("the PNG icons are real PNGs of the size they claim", () => {
    for (const size of [192, 512]) {
      const png = readFileSync(pub(`icon-${size}.png`));
      expect(png.subarray(1, 4).toString()).toBe("PNG");
      expect([png.readUInt32BE(16), png.readUInt32BE(20)]).toEqual([size, size]);
    }
  });
});
