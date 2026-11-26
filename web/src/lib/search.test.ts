import { describe, expect, it } from "vitest";
import type { Category, Member, PaymentMethod } from "../api/types";
import { matcher } from "./search";

const categories = [{ id: 1, name: "Moradia" }] as Category[];
const paymentMethods = [{ id: 2, name: "Crédito", card: { issuer: "Nubank", last4: "1234" } }] as PaymentMethod[];
const members = [{ userId: 7, name: "Rafa Souza" }] as Member[];
const item = { kind: "expense", amountCents: 123456, description: "Aluguel", categoryId: 1, paymentMethodId: 2, date: "2026-11-08", paidByUserId: 7 };
const finds = (q: string) => matcher(q, { categories, paymentMethods, members })(item);

describe("matcher", () => {
  it("finds by description, category, payment method, card issuer and last digits, ignoring accents and case", () => {
    for (const q of ["aluguel", "MORADIA", "credito", "nubank", "1234"]) expect(finds(q)).toBe(true);
  });
  it("finds by amount however it is typed", () => {
    for (const q of ["1.234,56", "1234,56", "1234.56", "R$ 1.234,56", "234,5"]) expect(finds(q)).toBe(true);
  });
  it("finds by date and by who paid", () => {
    for (const q of ["08/11", "8/11", "08/11/2026", "rafa"]) expect(finds(q)).toBe(true);
  });
  it("needs every word to match somewhere, and an empty search finds everything", () => {
    expect(finds("rafa moradia")).toBe(true);
    expect(finds("rafa mercado")).toBe(false);
    expect(finds("   ")).toBe(true);
  });
});
