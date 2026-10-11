# Importar o extrato do banco

Você baixa o extrato no banco, envia para o gastos e decide, linha por linha, o que vira lançamento. Nada é lançado sem a sua confirmação.

## Passo a passo

1. **Baixe o extrato** no app ou no internet banking. Procure por "exportar extrato" ou "baixar extrato" e escolha **OFX** (também chamado de "Money" ou "Quicken"). Se o seu banco só oferece CSV, serve também. Cada banco chama e organiza isso de um jeito, então não há um caminho único.
2. Em **Ajustes, Contas bancárias**, crie uma conta (por exemplo "Nubank cartão") e, se quiser, escolha a forma de pagamento que os lançamentos dela vão usar.
3. Clique em **Importar extrato**, escolha o formato e o arquivo.
4. Abra a **Caixa de entrada** e, para cada linha, escolha a categoria e clique em **Aceitar**, ou **Ignorar** o que não interessa.

Os lançamentos aceitos entram com a data e o valor do extrato, e quem paga é quem está definido na conta bancária.

## Formatos aceitos

- **OFX** (recomendado): traz data, valor e um identificador único por linha. Arquivos de até 5 MB.
- **CSV**: separadores `;`, `,` ou tabulação são detectados sozinhos. Valores como `1.234,56` e `1,234.56` são entendidos. Datas em `dd/mm/aaaa` ou `aaaa-mm-dd`. Você informa o número da coluna (a partir de 1) de data, descrição e valor, e se a primeira linha é cabeçalho.
  - Em fatura de cartão, as compras costumam aparecer como valores **positivos**. Marque "Compras aparecem como valores positivos" para o gastos tratá-las como gastos.

Valor negativo vira gasto e valor positivo vira entrada. Linhas de valor zero são puladas.

## Importar de novo é seguro

- Reenviar o mesmo arquivo, ou um extrato que se sobrepõe a outro, não duplica nada: linhas já vistas são contadas como "já importadas".
- Uma linha que você ignorou, ou cujo lançamento você apagou, não volta para a caixa.
- No CSV não há identificador por linha, então duas compras idênticas no mesmo dia continuam distintas, mas editar o arquivo à mão e reenviar pode trazer linhas de volta.

## Regras de categoria

Ao aceitar uma linha sem categoria sugerida, marque "Usar esta categoria nas próximas importações" e confirme o texto (por exemplo `uber`). Nas próximas importações, linhas cuja descrição contenha esse texto (sem diferenciar maiúsculas nem acentos) já chegam com a categoria escolhida. As regras ficam em Ajustes, Contas bancárias, e podem ser excluídas.

Uma regra de gasto nunca é aplicada a uma entrada, e vice-versa.

## Lançamentos que você já fez à mão

Se existe um lançamento do mesmo tipo e valor, com data até dois dias de diferença, a linha mostra "Parece o lançamento que você já fez". Use **É este lançamento** para ligar os dois sem criar um segundo. Esses casos ficam de fora de "Aceitar sugeridas", justamente para você decidir.

## Limitações

- Arquivos OFX variam entre bancos. O parser é tolerante, mas só foi testado com o formato padrão; se um arquivo for recusado, abra uma issue com um trecho anonimizado.
- Não há conexão automática com o banco (Open Finance). A importação é sempre manual.
- Só quem edita a carteira pode importar; leitores veem a caixa de entrada, mas não decidem.
