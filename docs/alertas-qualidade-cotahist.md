# Alertas de qualidade do COTAHIST

Desde o parser 1.2.0, fechamento abaixo da mínima ou acima da máxima gera
`close_outside_daily_range`, sem abortar a importação e sem alterar os preços.
O caso observado em BCFF11 em 08/06/2020 possui mínima de 90,00, máxima de 92,50
e último preço de 89,61 no próprio arquivo de origem.

O importador emite um log `WARN` por registro afetado com código, importação,
arquivo, linha, hash da linha, ticker, mercado, data e preços em centavos.
Erros estruturais e as demais validações continuam fatais, incluindo fechamento
negativo, máxima menor que mínima e abertura fora da faixa.

Nesta alteração, o alerta é registrado nos logs; não foi adicionada uma coluna
de qualidade nem modificada a resposta da `market-data-api`. Como os valores
originais são preservados, registros publicados com a divergência podem ser
identificados diretamente no banco:

```sql
SELECT import_id, line_number, ticker, trading_date, market_type,
       close_price, low_price, high_price,
       'close_outside_daily_range' AS quality_code
FROM published_historical_quotes
WHERE close_price < low_price OR close_price > high_price;
```

A integração dessa sinalização nos indicadores da API é uma etapa separada.
Um alerta de ingestão não comprova que o fechamento está correto para análise.

## Reexecução de 2020

```powershell
go run .\cmd\main.go 2020
```

O fluxo existente remove os registros parciais ao marcar falha e também limpa
os registros da importação ao reiniciar um checksum não publicado. A retomada
agora atualiza as versões de parser e layout. Um checksum já publicado continua
sendo ignorado, conforme o comportamento existente.

Não executar duas importações simultâneas do mesmo arquivo. Nenhuma importação
ou alteração no banco foi executada como parte desta correção.

## Validação

Testes cobrem fechamento abaixo/acima da faixa, limites válidos, preservação de
preços, manutenção dos erros fatais, emissão do WARN e continuidade da persistência
com repositório simulado. A leitura completa do ZIP local de 2020 processou
1.251.646 registros e identificou 280 alertas, sem gravar no banco.

Para repetir a leitura local:

```powershell
$env:COTAHIST_LOCAL_SCAN = '1'
go test ./internal/adapters/outbound/cotahist -run '^TestLocal2020QualityScan$' -count=1 -v
```
