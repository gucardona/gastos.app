package wallet

import (
	"context"

	"gastos/internal/api/httpx"
)

// seedWallet creates the default categories and payment methods of a new wallet.
func seedWallet(ctx context.Context, q httpx.Querier, walletID int64) error {
	expense := []struct{ name, icon, color, essentiality string }{
		{"Alimentação", "🍽", "#0f6579", "essential"},
		{"Mercado", "🛒", "#1f7d8f", "essential"},
		{"Transporte", "🚗", "#2b92a1", "essential"},
		{"Saúde", "💊", "#27ae60", "essential"},
		{"Moradia", "🏠", "#2c9b88", "essential"},
		{"Contas", "⚡", "#d6a23a", "essential"},
		{"Restaurante", "🍜", "#55b38c", "nonessential"},
		{"Lazer", "🎮", "#66c3a1", "nonessential"},
		{"Assinaturas", "📺", "#8e44ad", "nonessential"},
		{"Vestuário", "👕", "#88c0b3", "nonessential"},
		{"Beleza", "✨", "#79b7ae", "nonessential"},
		{"Viagem", "✈️", "#54a7b9", "nonessential"},
		{"Educação", "📚", "#2ba18a", "investment"},
		{"Investimentos", "📈", "#f39c12", "investment"},
		{"Outros", "◈", "#7f9ea5", "nonessential"},
	}
	for i, c := range expense {
		if _, err := q.ExecContext(ctx, `
			INSERT INTO categories (wallet_id, kind, name, icon, color, essentiality, sort_order)
			VALUES (?, 'expense', ?, ?, ?, ?, ?)`, walletID, c.name, c.icon, c.color, c.essentiality, i); err != nil {
			return err
		}
	}
	income := []struct{ name, icon, color string }{
		{"Salário", "💼", "#0f6579"},
		{"Freelance", "🧑‍💻", "#1f7d8f"},
		{"Investimentos", "📈", "#f39c12"},
		{"Reembolso", "↩️", "#2b92a1"},
		{"Outros", "◈", "#7f9ea5"},
	}
	for i, c := range income {
		if _, err := q.ExecContext(ctx, `
			INSERT INTO categories (wallet_id, kind, name, icon, color, sort_order)
			VALUES (?, 'income', ?, ?, ?, ?)`, walletID, c.name, c.icon, c.color, i); err != nil {
			return err
		}
	}
	methods := []struct{ name, icon string }{
		{"Pix", "⚡"}, {"Crédito", "💳"}, {"Débito", "🏦"}, {"Dinheiro", "💵"},
		{"Boleto", "📄"}, {"Vale Alim.", "🍽"}, {"Vale Ref.", "🛒"}, {"Transferência", "↔"},
	}
	for i, m := range methods {
		if _, err := q.ExecContext(ctx, `
			INSERT INTO payment_methods (wallet_id, name, icon, sort_order) VALUES (?, ?, ?, ?)`,
			walletID, m.name, m.icon, i); err != nil {
			return err
		}
	}
	return nil
}
