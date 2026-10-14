import { expect, test, type Browser, type Locator, type Page } from "@playwright/test";

// Each test creates new users: unique emails avoid depending on execution order.
const unique = (name: string) => `${name.toLowerCase()}.${Date.now()}.${Math.floor(Math.random() * 1e6)}@example.com`;
const password = "senha-de-teste-123";
// Sign-up is rate limited per IP (10 per 15 minutes) and every test here signs up, so each browser context
// gets its own address; the server trusts X-Forwarded-For from loopback.
const fakeIP = () => ({ "X-Forwarded-For": `10.${Math.floor(Math.random() * 250)}.${Math.floor(Math.random() * 250)}.${1 + Math.floor(Math.random() * 250)}` });
test.beforeEach(async ({ context }) => context.setExtraHTTPHeaders(fakeIP()));

const inviteCode = "e2e-invite-code"; // same value as GASTOS_INVITE_CODE in playwright.config.ts

async function signUp(page: Page, name: string, email = unique(name), code = inviteCode) {
  await page.goto("/");
  await expect(page).toHaveURL(/\/login$/); // without a session, it lands on the login
  // the toggle only shows once the server has said how sign-up works, so the field decision below is stable
  await page.getByRole("button", { name: "Criar uma conta" }).click();
  await page.getByLabel("Como você gostaria de ser chamado?").fill(name);
  await page.getByLabel("E-mail").fill(email);
  await page.getByLabel("Senha", { exact: true }).fill(password);
  await page.getByLabel("Repita a senha").fill(password);
  // the very first account of a fresh database needs no code; every later one does
  const codeField = page.getByLabel("Código de convite");
  if (await codeField.count()) await codeField.fill(code);
  await page.getByRole("button", { name: "Criar conta" }).click();
  return email;
}

/** signUp, then waits for the overview. */
async function signUpAndEnter(page: Page, name: string) {
  const email = await signUp(page, name);
  await expect(page.getByText("Saldo do mês")).toBeVisible();
  return email;
}

/** Opens the Categoria row of the quick-add sheet and taps a category; the picker closes by itself. */
async function pickCategory(sheet: Locator, name: string) {
  await sheet.getByRole("button", { name: /^Categoria/ }).click();
  await sheet.getByRole("button", { name: new RegExp(name) }).click();
}

async function addExpense(page: Page, amount: string, description: string, category = "Mercado") {
  await page.getByRole("button", { name: "Novo lançamento" }).first().click();
  const sheet = page.getByRole("dialog");
  await sheet.getByRole("textbox", { name: "Valor" }).fill(amount);
  await sheet.getByLabel("Descrição").fill(description);
  await pickCategory(sheet, category);
  await sheet.getByRole("button", { name: "Salvar", exact: true }).click();
  await expect(sheet).toBeHidden();
}

test("sign up, add, edit and delete a transaction", async ({ page }) => {
  await signUpAndEnter(page, "Ana");
  await expect(page.getByText(/Nada lançado em Pessoal/)).toBeVisible();

  await addExpense(page, "48,90", "Mercado do bairro");
  await expect(page.getByText("−R$ 48,90").first()).toBeVisible();
  await expect(page.getByText("Mercado do bairro")).toBeVisible();
  await expect(page.getByText("Saiu").locator("..")).toContainText("48,90");

  // edit from the transactions list
  await page.getByRole("link", { name: /Lançamentos/ }).first().click();
  await page.getByRole("button", { name: /Mercado do bairro/ }).click();
  await page.getByRole("textbox", { name: "Valor" }).fill("50,00");
  await page.getByRole("button", { name: "Salvar", exact: true }).click();
  await expect(page.getByText("−R$ 50,00").first()).toBeVisible();

  // delete, with confirmation
  await page.getByRole("button", { name: /Mercado do bairro/ }).click();
  await page.getByRole("button", { name: "Excluir lançamento" }).click();
  await page.getByRole("button", { name: "Confirmar exclusão" }).click();
  await expect(page.getByText("Nenhum lançamento neste mês.")).toBeVisible();
});

test("session: sign out and back in, with a wrong password rejected", async ({ page }) => {
  const email = await signUpAndEnter(page, "Caio");
  await page.getByRole("link", { name: "Ajustes" }).first().click();
  await page.getByRole("button", { name: "Sair" }).click();
  await expect(page).toHaveURL(/\/login$/);

  await page.getByLabel("E-mail").fill(email);
  await page.getByLabel("Senha", { exact: true }).fill("errada-errada");
  await page.getByRole("button", { name: "Entrar" }).click();
  await expect(page.getByText("E-mail ou senha incorretos")).toBeVisible();

  await page.getByLabel("Senha", { exact: true }).fill(password);
  await page.getByRole("button", { name: "Entrar" }).click();
  await expect(page.getByText("Saldo do mês")).toBeVisible();
});

// Two people, each in their own browser, on Ana's wallet with splitting enabled.
async function pairUp(browser: Browser) {
  const ana = await newPage(browser);
  const bia = await newPage(browser);
  await signUpAndEnter(ana, "Ana");
  const biaEmail = await signUpAndEnter(bia, "Bia");

  // Ana invites Bia as an editor and enables splitting, through her own session's API
  const me = await (await ana.request.get("/api/me")).json();
  const walletId = me.wallets[0].id;
  const invite = await ana.request.post(`/api/wallets/${walletId}/members`, { data: { email: biaEmail, role: "editor" } });
  expect(invite.status()).toBe(201);
  await ana.request.patch(`/api/wallets/${walletId}`, { data: { splittingEnabled: true } });

  // Bia reloads, sees Ana's wallet in the selector and starts using it
  await bia.reload();
  const biaWallets = await (await bia.request.get("/api/me")).json();
  const shared = biaWallets.wallets.find((w: { ownerName: string }) => w.ownerName.startsWith("Ana"));
  await bia.locator("select[aria-label=Carteira]:visible").selectOption(String(shared.id)); // the other layout's selector is in the DOM but hidden
  await ana.reload();
  await expect(ana.locator("small:visible", { hasText: /com Bia/ })).toHaveCount(1); // the wallet label also exists, hidden, in the other layout
  return { ana, bia };
}

test("real time: one person's expense shows up for the other without reloading", async ({ browser }) => {
  const { ana, bia } = await pairUp(browser);
  await addExpense(bia, "380,00", "Jantar no Ida", "Mercado");

  // Ana does not reload: SSE updates the screen
  await expect(ana.getByText("Jantar no Ida")).toBeVisible({ timeout: 10_000 });
  await expect(ana.getByText("Acerto entre vocês")).toBeVisible();
  await expect(ana.getByText(/Você deve esse valor para Bia/)).toBeVisible();
});

test("splitting: balance, suggested settlement and history", async ({ browser }) => {
  const { ana, bia } = await pairUp(browser);
  await addExpense(bia, "380,00", "Jantar no Ida", "Mercado");

  await ana.getByRole("link", { name: /Divisão/ }).first().click();
  await expect(ana.getByText("Saldo entre vocês")).toBeVisible();
  await expect(ana.getByText("Você deve esse valor para Bia.")).toBeVisible();
  await expect(ana.getByText(/−R\$\s*190,00/).first()).toBeVisible();

  // Ana settles up: the balance goes to zero and the settlement enters the history
  await ana.getByRole("button", { name: "Registrar acerto" }).click();
  await expect(ana.getByText("Vocês estão quites.")).toBeVisible();
  await expect(ana.getByText(/Você pagou Bia/)).toBeVisible();

  // Bia sees the same, without reloading
  await bia.getByRole("link", { name: /Divisão/ }).first().click();
  await expect(bia.getByText("Vocês estão quites.")).toBeVisible();
  await expect(bia.getByText(/Ana pagou a você/)).toBeVisible();

  // deleting the settlement brings the balance back
  await ana.getByRole("button", { name: "Excluir" }).click();
  await ana.getByRole("button", { name: "Confirmar exclusão" }).click();
  await expect(ana.getByText("Você deve esse valor para Bia.")).toBeVisible();
});

// browser.newContext() does not inherit the project's device settings, so pass them along (otherwise the mobile project would run these on a desktop viewport)
async function newPage(browser: Browser) {
  const context = await browser.newContext({ ...test.info().project.use, extraHTTPHeaders: fakeIP() });
  return context.newPage();
}

test("settings and recurring: category, overspent budget and paused recurrence", async ({ page }) => {
  await signUpAndEnter(page, "Dani");

  // new category
  await page.getByRole("link", { name: "Ajustes" }).first().click();
  await page.getByRole("link", { name: "Categorias" }).click();
  await page.getByRole("button", { name: "Nova categoria" }).click();
  const category = page.getByRole("dialog");
  await category.getByLabel("Nome da categoria").fill("Pets");
  await category.getByRole("button", { name: "Outros" }).click();
  await category.getByRole("button", { name: "🐶" }).click();
  await category.getByRole("button", { name: "Cor #D4568B" }).click();
  await category.getByRole("button", { name: "Criar categoria" }).click();
  await expect(category).toBeHidden();
  await expect(page.getByRole("button", { name: /Pets/ })).toBeVisible();

  // a R$ 100,00 budget on Restaurante
  await page.goto("/ajustes/orcamentos");
  await page.getByRole("button", { name: /Restaurante/ }).click();
  await page.getByRole("dialog").getByLabel("Limite mensal de Restaurante").fill("100,00");
  await page.getByRole("dialog").getByRole("button", { name: "Definir" }).click();
  await expect(page.getByRole("dialog")).toBeHidden();
  await expect(page.getByRole("button", { name: /Restaurante/ })).toContainText("100,00");

  // a R$ 130,00 expense overspends the budget on the overview
  await page.getByRole("link", { name: /Visão/ }).first().click();
  await addExpense(page, "130,00", "Jantar", "Restaurante");
  await expect(page.getByText("R$ 30,00 acima do orçamento")).toBeVisible();

  // a recurrence created from quick add and paused in Recorrentes
  await page.getByRole("button", { name: "Novo lançamento" }).first().click();
  const sheet = page.getByRole("dialog");
  await sheet.getByRole("textbox", { name: "Valor" }).fill("55,90");
  await sheet.getByLabel("Descrição").fill("Netflix");
  await pickCategory(sheet, "Lazer");
  await sheet.getByRole("switch", { name: "Repetir todo mês" }).click();
  await sheet.getByRole("button", { name: "Salvar", exact: true }).click();
  await expect(sheet).toBeHidden();

  await page.getByRole("link", { name: /Lançamentos/ }).first().click();
  await expect(page.getByRole("heading", { name: "Repetem todo mês" })).toBeVisible();
  const repeating = page.locator("section", { has: page.getByRole("heading", { name: "Repetem todo mês" }) });
  await expect(repeating.getByText("Netflix")).toBeVisible();
  await expect(repeating.getByText(/Todo dia \d+, próxima em/)).toBeVisible();
  await page.getByRole("switch", { name: "Pausar Netflix" }).click();
  await expect(repeating.getByText(/pausada/)).toBeVisible();

  // tapping the row edits it in a sheet
  await repeating.getByRole("button", { name: /Netflix/ }).click();
  await page.getByRole("dialog").getByLabel("Valor").fill("60,00");
  await page.getByRole("dialog").getByRole("button", { name: "Salvar", exact: true }).click();
  await expect(page.getByRole("dialog")).toBeHidden();
  await expect(repeating.getByText(/−R\$\s*60,00/)).toBeVisible();
});

test("layout follows the viewport: bottom nav on a phone, sidebar on a desktop", async ({ page, isMobile }) => {
  await signUpAndEnter(page, "Lia");
  const bottomNav = page.locator("nav.tabbar");
  const sidebar = page.locator("aside.sidebar");
  if (isMobile) {
    await expect(bottomNav).toBeVisible();
    await expect(sidebar).toBeHidden();
  } else {
    await expect(sidebar).toBeVisible();
    await expect(bottomNav).toBeHidden();
  }
});

test("invite code: a wrong code is refused, the right one creates the account", async ({ page }) => {
  await signUpAndEnter(page, "Dona"); // makes sure an account exists, so the next sign-ups need the code
  await page.getByRole("link", { name: "Ajustes" }).first().click();
  await page.getByRole("button", { name: "Sair" }).click();
  await expect(page).toHaveURL(/\/login$/);

  await signUp(page, "Intruso", unique("Intruso"), "not-the-right-code");
  await expect(page.getByText("O cadastro está fechado. Peça um convite.")).toBeVisible();
  await expect(page.getByText("Saldo do mês")).toBeHidden();

  // same form, now with the right code
  await page.getByLabel("Código de convite").fill(inviteCode);
  await page.getByRole("button", { name: "Criar conta" }).click();
  await expect(page.getByText("Saldo do mês")).toBeVisible();
});

test("import an OFX statement, accept a line and ignore another", async ({ page }) => {
  await signUpAndEnter(page, "Dora");
  const d = new Date();
  const day = `${d.getFullYear()}${String(d.getMonth() + 1).padStart(2, "0")}${String(d.getDate()).padStart(2, "0")}`;
  const ofx = `OFXHEADER:100\n\n<OFX><BANKTRANLIST>
<STMTTRN><TRNTYPE>X<DTPOSTED>${day}<TRNAMT>-48.90<FITID>A1<MEMO>COMPRA MERCADO DO BAIRRO</STMTTRN>
<STMTTRN><TRNTYPE>X<DTPOSTED>${day}<TRNAMT>-9.50<FITID>A2<MEMO>TARIFA BANCARIA</STMTTRN>
</BANKTRANLIST></OFX>`;

  await page.getByRole("link", { name: "Ajustes" }).first().click();
  await page.getByRole("link", { name: "Contas bancárias" }).click();
  await page.getByRole("button", { name: "Nova conta bancária" }).click();
  await page.getByLabel("Nome da conta bancária").fill("Conta corrente");
  await page.getByRole("button", { name: "Criar", exact: true }).click();
  await expect(page.getByRole("dialog")).toBeHidden();
  await page.getByRole("button", { name: /Conta corrente/ }).click();
  await page.getByLabel("Arquivo do extrato").setInputFiles({ name: "extrato.ofx", mimeType: "application/x-ofx", buffer: Buffer.from(ofx) });
  await page.getByRole("button", { name: "Importar", exact: true }).click();
  await expect(page.getByText("2 linhas novas, 0 já importadas antes.")).toBeVisible();

  // sending the same file again brings nothing new
  await page.getByLabel("Arquivo do extrato").setInputFiles({ name: "extrato.ofx", mimeType: "application/x-ofx", buffer: Buffer.from(ofx) });
  await page.getByRole("button", { name: "Importar", exact: true }).click();
  await expect(page.getByText("0 linhas novas, 2 já importadas antes.")).toBeVisible();
  await page.getByRole("dialog").getByRole("button", { name: "Fechar" }).click();

  await page.getByRole("link", { name: /Caixa/ }).first().click();
  await page.getByRole("button", { name: /COMPRA MERCADO DO BAIRRO/ }).click();
  const decision = page.getByRole("dialog");
  await pickCategory(decision, "Mercado");
  await decision.getByRole("button", { name: "Aceitar" }).click();
  await expect(decision).toBeHidden();
  await page.getByRole("button", { name: /TARIFA BANCARIA/ }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Ignorar" }).click();
  await expect(page.getByText(/Nada para revisar/)).toBeVisible();

  await page.getByRole("link", { name: /Lançamentos/ }).first().click();
  await expect(page.getByText("COMPRA MERCADO DO BAIRRO")).toBeVisible();
  await expect(page.getByText("TARIFA BANCARIA")).toHaveCount(0);
});

test("navigation: three destinations, one way to Ajustes, and the + button", async ({ page }) => {
  await signUpAndEnter(page, "Nina");
  const nav = page.getByRole("navigation", { name: "Navegação" });
  await expect(nav.getByRole("link")).toHaveText([/Visão/, /Lançamentos/, /Caixa/]); // no Divisão without splitting, no Recorrentes
  await expect(page.locator("select[aria-label=Carteira]:visible")).toHaveCount(1);
  await expect(page.getByRole("link", { name: "Ajustes" })).toHaveCount(1);

  await page.getByRole("button", { name: "Novo lançamento" }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.keyboard.press("Escape");

  await page.getByRole("link", { name: "Ajustes" }).click();
  await expect(page).toHaveURL(/\/ajustes/);
});

test("quick add: pickers for category and day, and Esc leaves without saving", async ({ page }) => {
  await signUpAndEnter(page, "Rita");
  await page.getByRole("button", { name: "Novo lançamento" }).click();
  const sheet = page.getByRole("dialog");
  await sheet.getByRole("textbox", { name: "Valor" }).fill("48,90");
  await sheet.getByLabel("Descrição").fill("Padaria de ontem");
  await pickCategory(sheet, "Restaurante");
  await sheet.getByRole("button", { name: /^Quando/ }).click();
  await sheet.getByRole("button", { name: "Ontem" }).click();
  await expect(sheet.getByRole("button", { name: /^Quando/ })).toContainText("Ontem");
  await sheet.getByRole("button", { name: "Salvar", exact: true }).click();
  await expect(sheet).toBeHidden();

  await page.getByRole("link", { name: /Lançamentos/ }).first().click();
  await expect(page.getByText("Padaria de ontem")).toBeVisible();
  await expect(page.getByText(/Ontem,/)).toBeVisible();

  await page.getByRole("button", { name: "Novo lançamento" }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toBeHidden();
  await expect(page.getByText("Padaria de ontem")).toHaveCount(1);
});

test("the overview does not scroll sideways at the narrowest desktop width, even with large amounts", async ({ page, isMobile }) => {
  test.skip(isMobile, "desktop layout only");
  await page.setViewportSize({ width: 900, height: 800 });
  await signUpAndEnter(page, "Vera");
  await page.evaluate(async () => {
    const j = async (u: string, m = "GET", b?: unknown) => (await fetch(u, { method: m, headers: { "Content-Type": "application/json" }, body: b ? JSON.stringify(b) : undefined })).json();
    const w = (await j("/api/me")).wallets[0].id;
    const cats = await j(`/api/wallets/${w}/categories`);
    const iso = new Date().toISOString().slice(0, 10);
    await j(`/api/wallets/${w}/transactions`, "POST", { kind: "income", amountCents: 1820000, description: "Salário", categoryId: cats.find((c: { kind: string }) => c.kind === "income").id, date: iso, paymentMethodId: null });
    await j(`/api/wallets/${w}/transactions`, "POST", { kind: "expense", amountCents: 585433, description: "Aluguel", categoryId: cats.find((c: { kind: string }) => c.kind === "expense").id, date: iso, paymentMethodId: null });
  });
  await page.reload();
  await expect(page.getByText("Para onde foi")).toBeVisible();
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(overflow).toBeLessThanOrEqual(0);
});

test("the old /recorrentes address lands on Lançamentos", async ({ page }) => {
  await signUpAndEnter(page, "Olga");
  await page.goto("/recorrentes");
  await expect(page).toHaveURL(/lancamentos$/);
  await expect(page.getByRole("heading", { name: "Repetem todo mês" })).toBeVisible();
});

test("long descriptions never make Lançamentos or the Caixa scroll sideways", async ({ page, isMobile }) => {
  if (!isMobile) await page.setViewportSize({ width: 900, height: 800 }); // the narrowest width of the desktop layout
  await signUpAndEnter(page, "Zeca");
  await page.evaluate(async () => {
    const j = async (u: string, m = "GET", b?: unknown) => (await fetch(u, { method: m, headers: { "Content-Type": "application/json" }, body: b ? JSON.stringify(b) : undefined })).json();
    const w = (await j("/api/me")).wallets[0].id;
    const user = (await j("/api/me")).user.id;
    const cats = await j(`/api/wallets/${w}/categories`);
    const cat = cats.find((c: { kind: string }) => c.kind === "expense").id;
    const iso = new Date().toISOString().slice(0, 10);
    const long = "PAG*JOSEDASILVACOMERCIODEALIMENTOSLTDA".repeat(4); // one unbroken token, as banks send them
    await j(`/api/wallets/${w}/transactions`, "POST", { kind: "expense", amountCents: 123456789, description: long, categoryId: cat, date: iso, paymentMethodId: null });
    await j(`/api/wallets/${w}/recurrences`, "POST", { kind: "expense", amountCents: 5590, description: long, categoryId: cat, paymentMethodId: null, paidByUserId: user, dayOfMonth: 5, startDate: iso });
    const acc = await j(`/api/wallets/${w}/bank-accounts`, "POST", { name: "Conta com um nome bem comprido para testar" });
    const ofx = `OFXHEADER:100\n\n<OFX><BANKTRANLIST>\n<STMTTRN><TRNTYPE>X<DTPOSTED>${iso.replace(/-/g, "")}<TRNAMT>-9999999.99<FITID>L1<MEMO>${long}</STMTTRN>\n</BANKTRANLIST></OFX>`;
    const f = new FormData();
    f.set("format", "ofx");
    f.set("file", new File([ofx], "x.ofx"));
    await fetch(`/api/wallets/${w}/bank-accounts/${acc.id}/import`, { method: "POST", body: f });
  });
  for (const path of ["/lancamentos", "/caixa"]) {
    await page.goto(path);
    await expect(page.getByText(/PAG\*JOSEDASILVA/).first()).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth), path).toBeLessThanOrEqual(0);
  }
});

test("old Ajustes addresses still work, and inviting someone who has no account says so", async ({ page }) => {
  await signUpAndEnter(page, "Iara");
  await page.goto("/ajustes?s=bancos");
  await expect(page).toHaveURL(/\/ajustes\/bancos$/);
  await expect(page.getByRole("button", { name: "Nova conta bancária" })).toBeVisible();

  await page.goto("/ajustes/pessoas");
  await page.getByRole("button", { name: "Convidar alguém" }).click();
  await page.getByLabel("E-mail do convidado").fill("ninguem@example.com");
  await page.getByRole("button", { name: "Convidar", exact: true }).click();
  await expect(page.getByRole("dialog").getByRole("alert")).toBeVisible();
});

test("no Ajustes screen scrolls sideways, even with long names", async ({ page, isMobile }) => {
  if (!isMobile) await page.setViewportSize({ width: 900, height: 800 });
  await signUpAndEnter(page, "Jair");
  const long = "UmNomeMuitoLongoSemEspacosParaTestarOTransbordoDaLista".repeat(2);
  await page.evaluate(async (name) => {
    const j = async (u: string, m = "GET", b?: unknown) => (await fetch(u, { method: m, headers: { "Content-Type": "application/json" }, body: b ? JSON.stringify(b) : undefined })).json();
    const w = (await j("/api/me")).wallets[0].id;
    await j(`/api/wallets/${w}/categories`, "POST", { kind: "expense", name: name.slice(0, 40), icon: "🐶", color: "#0E6577", essentiality: "essential" });
    await j(`/api/wallets/${w}/bank-accounts`, "POST", { name: name.slice(0, 60) });
    await j(`/api/wallets/${w}/payment-methods`, "POST", { name: name.slice(0, 40), icon: "💳" });
  }, long);
  for (const section of ["perfil", "carteira", "pessoas", "categorias", "pagamentos", "orcamentos", "bancos", "recorrentes", "aparencia"]) {
    await page.goto(`/ajustes/${section}`);
    await expect(page.getByRole("heading", { level: 1 }).first()).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth), section).toBeLessThanOrEqual(0);
  }
});
