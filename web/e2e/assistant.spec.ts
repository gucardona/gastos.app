import { expect, test, type Page } from "@playwright/test";

// Same sign-up helpers as app.spec.ts (sign-up is rate limited per IP, so each context fakes its own address).
const unique = (name: string) => `${name.toLowerCase()}.${Date.now()}.${Math.floor(Math.random() * 1e6)}@example.com`;
const password = "senha-de-teste-123";
const fakeIP = () => ({ "X-Forwarded-For": `10.${Math.floor(Math.random() * 250)}.${Math.floor(Math.random() * 250)}.${1 + Math.floor(Math.random() * 250)}` });
test.beforeEach(async ({ context }) => context.setExtraHTTPHeaders(fakeIP()));

const inviteCode = "e2e-invite-code";
const fakeKey = "sk-ant-e2e-fake-key-0000"; // the only key the fake Anthropic server accepts

async function signUpAndEnter(page: Page, name: string) {
  await page.goto("/");
  await page.getByRole("button", { name: "Criar uma conta" }).click();
  await page.getByLabel("Como você gostaria de ser chamado?").fill(name);
  await page.getByLabel("E-mail").fill(unique(name));
  await page.getByLabel("Senha", { exact: true }).fill(password);
  await page.getByLabel("Repita a senha").fill(password);
  const codeField = page.getByLabel("Código de convite");
  if (await codeField.count()) await codeField.fill(inviteCode);
  await page.getByRole("button", { name: "Criar conta" }).click();
  await expect(page.getByText("Saldo do mês")).toBeVisible();
}

async function openAssistantSettings(page: Page) {
  await page.getByRole("link", { name: "Ajustes" }).first().click();
  await page.getByRole("link", { name: /Assistente/ }).click();
}

test("assistente: setup, chat with a chart, reload, new conversation, wrong key", async ({ page }) => {
  await signUpAndEnter(page, "Ana");
  const chatTab = page.getByRole("link", { name: "Chat" });
  await expect(chatTab).toHaveCount(0); // no key yet

  await openAssistantSettings(page);
  // a wrong key is refused with the error under the field
  await page.getByLabel("Chave da API").fill("sk-ant-errada");
  await page.getByRole("button", { name: "Salvar chave" }).click();
  await expect(page.getByRole("alert")).toBeVisible();
  await expect(page.getByText(/^•••• /)).toHaveCount(0);

  await page.getByLabel("Chave da API").fill(fakeKey);
  await page.getByRole("button", { name: "Salvar chave" }).click();
  await expect(page.getByText(/^•••• /)).toBeVisible();
  await page.getByRole("button", { name: "Sonnet" }).click();
  await expect(page.getByRole("button", { name: "Sonnet" })).toHaveAttribute("aria-pressed", "true");
  await page.getByRole("switch", { name: "Ativar chat" }).click();
  await expect(page.getByRole("switch", { name: "Ativar chat" })).toHaveAttribute("aria-checked", "true");

  await chatTab.first().click();
  await expect(page).toHaveURL(/\/chat$/);
  await page.getByLabel("Mensagem").fill("quanto gastei este mês?");
  await page.getByRole("button", { name: "Enviar" }).click();
  await expect(page.getByText("Você gastou R$ 150,00 no total, 60% em Mercado.")).toBeVisible();
  await expect(page.getByRole("img", { name: /Gastos por categoria.*Mercado.*60%/ })).toBeVisible();

  // wallet picker: current wallet shown, "Selecionar todas" offered
  await page.getByRole("button", { name: /Pessoal/ }).click();
  await page.getByRole("button", { name: "Selecionar todas" }).click();
  await page.getByRole("button", { name: "Fechar" }).click();

  await page.reload();
  await expect(page.getByText("quanto gastei este mês?")).toBeVisible();
  await expect(page.getByText("Você gastou R$ 150,00 no total, 60% em Mercado.")).toBeVisible();

  await page.getByRole("button", { name: "Nova conversa" }).click();
  await expect(page.getByText("quanto gastei este mês?")).toHaveCount(0);
  await expect(page.getByRole("img", { name: /Gastos por categoria/ })).toHaveCount(0);

  // other questions get plain text
  await page.getByLabel("Mensagem").fill("oi");
  await page.getByRole("button", { name: "Enviar" }).click();
  await expect(page.getByText("Posso ajudar com os seus gastos.")).toBeVisible();
});
