import { expect, test, type Browser, type Page } from "@playwright/test";

// Cada teste cria usuários novos: e-mails únicos evitam depender da ordem de execução.
const unique = (name: string) => `${name.toLowerCase()}.${Date.now()}.${Math.floor(Math.random() * 1e6)}@example.com`;
const password = "senha-de-teste-123";

async function signUp(page: Page, name: string, email = unique(name)) {
  await page.goto("/");
  await expect(page).toHaveURL(/\/login$/); // sem sessão, cai no login
  await page.getByRole("button", { name: "Criar uma conta" }).click();
  await page.getByLabel("Nome").fill(name);
  await page.getByLabel("E-mail").fill(email);
  await page.getByLabel("Senha").fill(password);
  await page.getByRole("button", { name: "Criar conta" }).click();
  await expect(page.getByText("Saldo do mês")).toBeVisible();
  return email;
}

async function addExpense(page: Page, amount: string, description: string, category = "Mercado") {
  await page.getByRole("button", { name: "Novo lançamento" }).first().click();
  const sheet = page.getByRole("dialog");
  await sheet.getByRole("textbox", { name: "Valor" }).fill(amount);
  await sheet.getByLabel("Descrição").fill(description);
  await sheet.getByRole("button", { name: new RegExp(category) }).click();
  await sheet.getByRole("button", { name: "Salvar", exact: true }).click();
  await expect(sheet).toBeHidden();
}

test("cadastro, lançamento, edição e exclusão", async ({ page }) => {
  await signUp(page, "Ana");
  await expect(page.getByText(/Nada lançado em Pessoal/)).toBeVisible();

  await addExpense(page, "48,90", "Mercado do bairro");
  await expect(page.getByText("−R$ 48,90").first()).toBeVisible();
  await expect(page.getByText("Mercado do bairro")).toBeVisible();
  await expect(page.getByText("Saiu").locator("..")).toContainText("48,90");

  // edita pela lista de lançamentos
  await page.getByRole("link", { name: /Lançamentos/ }).first().click();
  await page.getByRole("button", { name: /Mercado do bairro/ }).click();
  await page.getByRole("textbox", { name: "Valor" }).fill("50,00");
  await page.getByRole("button", { name: "Salvar", exact: true }).click();
  await expect(page.getByText("−R$ 50,00").first()).toBeVisible();

  // exclui, com confirmação
  await page.getByRole("button", { name: /Mercado do bairro/ }).click();
  await page.getByRole("button", { name: "Excluir lançamento" }).click();
  await page.getByRole("button", { name: "Confirmar exclusão" }).click();
  await expect(page.getByText("Nenhum lançamento neste mês.")).toBeVisible();
});

test("sessão: sair e entrar de novo, com senha errada recusada", async ({ page }) => {
  const email = await signUp(page, "Caio");
  await page.getByRole("link", { name: "Ajustes" }).first().click();
  await page.getByRole("button", { name: "Sair" }).click();
  await expect(page).toHaveURL(/\/login$/);

  await page.getByLabel("E-mail").fill(email);
  await page.getByLabel("Senha").fill("errada-errada");
  await page.getByRole("button", { name: "Entrar" }).click();
  await expect(page.getByText("E-mail ou senha incorretos")).toBeVisible();

  await page.getByLabel("Senha").fill(password);
  await page.getByRole("button", { name: "Entrar" }).click();
  await expect(page.getByText("Saldo do mês")).toBeVisible();
});

test("tempo real: o gasto de uma pessoa aparece para a outra sem recarregar", async ({ browser }) => {
  const ana = await newPage(browser);
  const bia = await newPage(browser);
  await signUp(ana, "Ana");
  const biaEmail = await signUp(bia, "Bia");

  // Ana convida a Bia como editora e liga a divisão, pela API da própria sessão dela
  const me = await (await ana.request.get("/api/me")).json();
  const walletId = me.wallets[0].id;
  const invite = await ana.request.post(`/api/wallets/${walletId}/members`, { data: { email: biaEmail, role: "editor" } });
  expect(invite.status()).toBe(201);
  await ana.request.patch(`/api/wallets/${walletId}`, { data: { splittingEnabled: true } });

  // a Bia recarrega, vê a carteira da Ana no seletor e passa a usá-la
  await bia.reload();
  const biaWallets = await (await bia.request.get("/api/me")).json();
  const shared = biaWallets.wallets.find((w: { ownerName: string }) => w.ownerName.startsWith("Ana"));
  await bia.getByLabel("Carteira").selectOption(String(shared.id));
  await ana.reload();
  await expect(ana.getByText(/com Bia/)).toBeVisible();
  await addExpense(bia, "380,00", "Jantar no Ida", "Mercado");

  // a Ana não recarrega: o SSE atualiza a tela
  await expect(ana.getByText("Jantar no Ida")).toBeVisible({ timeout: 10_000 });
  await expect(ana.getByText("Acerto entre vocês")).toBeVisible();
  await expect(ana.getByText(/Você deve esse valor para Bia/)).toBeVisible();
});

async function newPage(browser: Browser) {
  const context = await browser.newContext();
  return context.newPage();
}
