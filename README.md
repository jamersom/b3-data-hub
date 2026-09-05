# B3 Data Hub

Projeto em Go para baixar, validar e armazenar arquivos de cotacoes historicas disponibilizados pela B3.

O projeto utiliza Ports and Adapters (arquitetura hexagonal). As regras de negócio ficam no domínio, a coordenação do fluxo e os contratos externos ficam na camada de aplicação, e as integrações tecnológicas ficam nos adapters.

## Fluxo de importacao

A importacao segue estas etapas:

1. `cmd/main.go` cria e conecta as dependencias.
2. `ImportHistoricalQuotesService` inicia o caso de uso.
3. `HistoricalQuoteSource` solicita o arquivo historico ao adapter da B3.
4. `HistoricalFile` valida o ano e a assinatura ZIP do conteudo.
5. `FileStore` envia o arquivo validado ao adapter de armazenamento.
6. `HistoricalQuoteRepository` registra a importação no PostgreSQL.
7. `HistoricalQuoteParser` interpreta os registros do arquivo COTAHIST.
8. As cotações são persistidas em lotes pelo adapter PostgreSQL.
9. O caso de uso valida header, trailer, contagem e regras do domínio.
10. A nova versão anual é publicada atomicamente e substitui a versão anterior.

```text
cmd/main.go
        |
        v
application/usecases
ImportHistoricalQuotesService
        |
        +--> HistoricalQuoteSource (outbound port)
        |          |
        |          v
        |     adapter/outbound/b3 -> HTTP B3
        |
        +--> HistoricalFile.Validate()
        |          |
        |          v
        |     domain
        |
        +--> FileStore (outbound port)
        |          |
        |          v
        |     adapter/outbound/storage -> disco local
        |
        +--> HistoricalQuoteParser (outbound port)
        |          |
        |          v
        |     adapter/outbound/cotahist -> registros
        |
        `--> HistoricalQuoteRepository (outbound port)
                   |
                   v
              adapter/outbound/postgres -> PostgreSQL
```

As interfaces em `internal/application/ports/outbound` descrevem as capacidades externas necessárias pelos casos de uso. O domínio permanece independente em `internal/domain`, e os tipos em `internal/adapters` fornecem as implementações concretas.

## Estrutura

```text
b3-data-hub/
|-- cmd/
|   `-- main.go
|-- internal/
|   |-- domain/
|   |-- application/
|   |   |-- ports/
|   |   |   `-- outbound/
|   |   `-- usecases/
|   |-- adapters/
|   |   `-- outbound/
|   |       |-- b3/
|   |       |-- cotahist/
|   |       |-- postgres/
|   |       |   |-- queries/
|   |       |   `-- sqlcgen/
|   |       `-- storage/
|   `-- infra/
|       |-- config/
|       |-- database/
|       `-- logger/
|-- migrations/
|-- docker-compose.yml
|-- docker-stack.yml
|-- go.mod
`-- README.md
```

### Responsabilidades

- `cmd`: ponto de entrada e composicao das dependencias.
- `internal/domain`: entidades e regras independentes de infraestrutura.
- `internal/application/ports/outbound`: contratos das dependências externas usadas pela aplicação.
- `internal/application/usecases`: coordenação dos casos de uso da aplicação.
- `internal/adapters/outbound/b3`: download HTTP do arquivo disponibilizado pela B3.
- `internal/adapters/outbound/cotahist`: parser dos registros fixos do arquivo COTAHIST.
- `internal/adapters/outbound/postgres`: implementacao do repositorio de cotacoes.
- `internal/adapters/outbound/postgres/queries`: comandos SQL mantidos manualmente.
- `internal/adapters/outbound/postgres/sqlcgen`: codigo Go tipado gerado pelo sqlc; nao deve ser editado manualmente.
- `internal/adapters/outbound/storage`: armazenamento do ZIP no disco local.
- `internal/infra/config`: leitura e validacao das variaveis de ambiente.
- `internal/infra/database`: criacao e verificacao do pool PostgreSQL.
- `internal/infra/logger`: configuração do logger estruturado da aplicação.

## Executando

### Ano especifico

```bash
go run ./cmd 2025
```

### Ano atual

Quando o ano nao e informado, a aplicacao utiliza o ano atual:

```bash
go run ./cmd
```

O arquivo e salvo por padrao em:

```text
./data/COTAHIST_A<ANO>.ZIP
```

Exemplo:

```text
./data/COTAHIST_A2025.ZIP
```

## Validacoes

Antes de armazenar o download, o dominio verifica:

- se o ano solicitado e igual ou posterior a 1986;
- se o conteudo possui pelo menos quatro bytes;
- se o conteudo inicia com uma assinatura reconhecida de arquivo ZIP.

Essas verificacoes evitam que respostas claramente invalidas, como uma pagina HTML, sejam armazenadas como arquivos historicos.

## Armazenamento seguro

O adapter local grava primeiro em um arquivo temporario com extensao `.part`:

```text
COTAHIST_A2025.ZIP.part
```

Depois de concluir a escrita, ele renomeia o arquivo para o nome definitivo. Isso reduz a possibilidade de um arquivo parcial ser tratado como download concluido.

## Testes

Execute todos os testes:

```bash
go test ./...
```

Execute tambem a analise estatica:

```bash
go vet ./...
```

## Evolucoes planejadas

- validar a integridade completa do ZIP com `archive/zip`;
- detectar respostas maiores que o limite configurado;
- permitir configurar o limite máximo do download por ambiente;
- implementar retry com backoff para falhas transitorias;
- disponibilizar uma API REST para consultas;
- executar importacoes automaticas por agendamento.

## PostgreSQL com Docker

O ambiente local utiliza PostgreSQL 17 e e configurado pelo arquivo `docker-compose.yml`.

Copie o arquivo de exemplo quando `.env` ainda nao existir:

```bash
cp .env.example .env
```

No PowerShell:

```powershell
Copy-Item .env.example .env
```

O projeto ja possui um `.env` local com credenciais exclusivas para desenvolvimento. Nao use essa senha em producao.

### Iniciar o banco

```bash
docker compose up -d --build
```

Acompanhe o estado do container:

```bash
docker compose ps
```

Acompanhe os logs:

```bash
docker compose logs -f postgres
```

### Importacao agendada

O serviço `importer-scheduler` permanece ativo com o `crond`. Os disparos ocorrem
às **19h, 20h, 21h, 22h, 23h, 00h, 03h, 06h e 09h**, no fuso
`America/Sao_Paulo`. Esses horários são uma política de tentativas do projeto;
não representam um horário de publicação confirmado pela B3.

```cron
0 0,3,6,9,19,20,21,22,23 * * * cd /app && /app/b3-data-hub --scheduled >> /proc/1/fd/1 2>> /proc/1/fd/2
```

Cada ciclo começa às 19h de um dia de pregão e termina às 09h do dia seguinte,
com até **nove tentativas**. Antes das 19h, a data de referência é a véspera:
a madrugada e a manhã de sábado continuam associadas ao pregão de sexta-feira.
Dias sem pregão não iniciam ciclos. O calendário considera fins de semana e
feriados do mercado listado da B3.

Antes de baixar, a aplicação consulta o PostgreSQL procurando cotações da data
esperada em uma importação **publicada**, do ano correspondente. Se encontrar,
encerra com `trading_date_already_published`, sem requisição de download à B3.
Esse controle usa o estado já persistido das importações e cotações: não precisa
de tabela adicional nem se perde quando o contêiner reinicia.

Exemplo: se às 19h e 20h o arquivo estiver desatualizado, mas às **21h** o pregão
esperado for importado e publicado, os disparos de 22h, 23h, 00h, 03h, 06h e 09h
apenas conferem o banco e encerram. O cron continua disparando; o download é
que deixa de ocorrer. Um checksum já conhecido, sozinho, não conclui o ciclo:
o pregão esperado precisa estar publicado.

Se houver HTTP 404, timeout ou outro erro, a execução registra a falha e o cron
tenta novamente no próximo horário. Um arquivo válido ainda desatualizado pode
ser importado pelo fluxo normal, mas mantém o ciclo pendente. Às 09h, se o download
falhar ou o arquivo continuar desatualizado, é registrado `scheduled import cycle
exhausted` em nível ERROR. Não há envio automático de e-mail ou mensagem. Os
horários perdidos enquanto o host/Docker estiver desligado não são recuperados
pelo cron.

Um advisory lock do PostgreSQL (`42330001`) impede sobreposição entre processos
agendados e importações manuais desta versão. O bloqueio cobre a verificação,
download, importação e publicação, usando uma conexão dedicada adicional ao pool.
A conexão é fechada ao final e o banco libera o bloqueio. Um processo agendado
que encontra o bloqueio ocupado encerra com `import_in_progress`.

O binário é construído pelo `Dockerfile`. O cron executa `cd /app`, e a imagem
define `DATA_DIR=/app/data`, corrigindo o antigo comportamento de salvar em
`/root/data`. No Docker Compose, `/app/data` está vinculado à pasta local `./data`.
No Docker Stack, configure um volume nesse caminho se precisar preservar os ZIPs
após substituir o contêiner; o controle de conclusão permanece no PostgreSQL.

#### Calendário de pregões

O arquivo `config/trading-calendar.json` contém os dias **sem pregão** por ano,
baseado nos calendários oficiais da B3 para
[2025](https://www.b3.com.br/pt_br/noticias/calendario-de-feriados-2025.htm) e
[2026](https://www.b3.com.br/pt_br/noticias/calendario-de-negociacao-da-b3-confira-o-funcionamento-da-bolsa-em-2026.htm).
Quarta-feira de Cinzas é dia de pregão; 9 de julho também tem negociação.

Atualize o JSON conforme o calendário oficial antes de entrar em um ano novo.
Se o ano do ciclo estiver ausente, o modo agendado falha explicitamente, em vez
de presumir um calendário. Para usar outro arquivo, defina
`TRADING_CALENDAR_PATH` com um caminho acessível ao processo. Na imagem, o padrão
é `/app/config/trading-calendar.json`; ao editar o calendário local, reconstrua
a imagem ou monte o arquivo atualizado nesse caminho.

#### Aplicar alterações do agendamento

Com o banco e as migrations já preparados:

```bash
docker compose up -d --build --no-deps importer-scheduler
```

Esse comando reconstrói e recria apenas o importador. Alterações no cron e no
calendário copiados para a imagem exigem essa reconstrução.

Acompanhe as importacoes:

```bash
docker compose logs -f importer-scheduler
```

Confira o horario e o agendamento dentro do container:

```bash
docker compose exec importer-scheduler date
docker compose exec importer-scheduler cat /etc/crontabs/root
```

Execute a verificação agendada imediatamente, respeitando o ciclo e a consulta
prévia ao banco:

```bash
docker compose exec -w /app importer-scheduler /app/b3-data-hub --scheduled
```

Para executar uma importação manual, inclusive para buscar correções posteriores
da B3 em um pregão já publicado, omita `--scheduled`:

```bash
docker compose exec -w /app importer-scheduler /app/b3-data-hub
docker compose exec -w /app importer-scheduler /app/b3-data-hub 2025
```

O modo manual mantém a verificação de checksum após o download e o bloqueio de
concorrência, mas não pula o download pela data de pregão. O modo agendado baixa
o arquivo anual do ano do ciclo, inclusive quando a execução cruza a virada do ano.

Testes do agendamento:

```bash
go test ./internal/domain ./internal/application/usecases
```

O teste de integração do controle de agendamento verifica o bloqueio e consulta
dados publicados sem alterar cotações. Requer banco configurado e populado. No
PowerShell, execute a partir da raiz do projeto:

```powershell
$env:DATABASE_INTEGRATION_TEST = '1'
go test ./internal/adapters/outbound/postgres -run TestScheduledImportStateIntegration -count=1 -v
```

O host do PostgreSQL dentro da rede Docker e `postgres`; `localhost` apontaria para o proprio container do importador.

### Conectar pelo terminal

```bash
docker compose exec postgres psql -U b3_user -d b3_data_hub
```

Dentro do `psql`, liste as tabelas:

```sql
\dt
```

Consulte os lotes e as cotacoes:

```sql
SELECT * FROM historical_imports;
SELECT * FROM historical_quotes LIMIT 10;
```

### Configuracao local

| Campo | Valor padrao |
|---|---|
| Host | `localhost` |
| Porta | `5432` |
| Banco | `b3_data_hub` |
| Usuario | `b3_user` |
| Senha | `b3_local_password` |

A string de conexao esta disponivel em `DATABASE_URL`:

```text
postgres://b3_user:b3_local_password@localhost:5432/b3_data_hub?sslmode=disable
```

### Pool de conexoes da aplicacao

A aplicacao carrega o `.env`, valida `DATABASE_URL` e testa a conexao com PostgreSQL durante a inicializacao.

| Variavel | Padrao | Finalidade |
|---|---:|---|
| `DB_MAX_CONNECTIONS` | `10` | Maximo de conexoes abertas pelo pool |
| `DB_MIN_CONNECTIONS` | `2` | Minimo de conexoes mantidas pelo pool |
| `DB_MAX_CONN_LIFETIME` | `30m` | Tempo maximo de vida de uma conexao |
| `DB_CONNECT_TIMEOUT` | `5s` | Limite para estabelecer uma conexao |

### Logs estruturados

A aplicacao escreve logs estruturados em `stdout`. O formato JSON e recomendado para containers e ferramentas de observabilidade; o formato texto facilita a leitura durante o desenvolvimento local.

| Variavel | Padrao | Valores |
|---|---|---|
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `json` | `json`, `text` |

Para acompanhar a persistencia de cada lote durante uma importacao:

```powershell
$env:LOG_LEVEL = "debug"
$env:LOG_FORMAT = "text"
go run ./cmd 2026
```

Os eventos incluem campos como `reference_year`, `import_id`, `file_sha256`, `size_bytes`, `records` e `duration`. Credenciais e o conteudo das cotacoes nao sao registrados.

O teste unitario nao exige PostgreSQL. Para executar o teste real de conexao no PowerShell:

```powershell
$env:DATABASE_INTEGRATION_TEST = "1"
$env:DATABASE_URL = "postgres://b3_user:b3_local_password@localhost:5432/b3_data_hub?sslmode=disable"
go test ./internal/adapters/outbound/postgres -run TestNewPoolIntegration -v
```
### Migrations iniciais

O Compose monta a migration abaixo em `/docker-entrypoint-initdb.d`:

```text
migrations/001_create_historical_quotes.up.sql
```

O PostgreSQL executa scripts desse diretorio somente quando cria um volume de dados vazio. Alterar a migration depois que o banco ja foi inicializado nao a executa novamente.

Para apagar o banco local, recriar o volume e executar a migration desde o inicio:

```bash
docker compose down -v
docker compose up -d
```

O comando `down -v` apaga permanentemente os dados locais do PostgreSQL.

Para apenas parar os containers preservando os dados:

```bash
docker compose down
```

## Persistencia das cotacoes

O processo completo de importacao segue este fluxo:

```text
Download do ZIP
    |
    v
Validacao do arquivo
    |
    v
Salvamento local
    |
    v
Calculo do SHA-256
    |
    v
Parser do TXT de 245 posicoes
    |
    v
Lotes de 10.000 registros
    |
    v
PostgreSQL via COPY
```

### Parser COTAHIST

O parser:

- abre o TXT diretamente dentro do ZIP;
- processa somente registros de detalhe do tipo `01`;
- exige exatamente 245 posicoes em cada registro;
- converte datas e campos numericos para tipos Go;
- remove os espacos de ticker, ISIN, nomes e outros campos textuais;
- converte `00000000` e `99991231` em vencimento nulo;
- mantem valores financeiros como inteiros escalados, evitando perda de precisao com `float64`;
- verifica se o ano das cotacoes corresponde ao ano do arquivo;
- valida a correspondencia entre header e trailer e a contagem declarada de registros;
- normaliza ticker e ISIN para maiusculas e converte a moeda `R$` para `BRL`;
- valida campos obrigatorios e a coerencia dos precos de abertura, maxima, minima e fechamento;
- calcula um SHA-256 para cada registro de detalhe;
- respeita cancelamento e timeout por `context.Context`.

O TXT descompactado e processado linha por linha. Ele nao e carregado por inteiro na memoria.

O download do ZIP também é feito em streaming para um arquivo temporário. Durante a cópia, a aplicação calcula o SHA-256 e rejeita respostas acima de 512 MiB. Depois da validação, o arquivo temporário é movido para `data`, evitando manter o ZIP completo na memória.

### Controle da importacao

Depois de validar e salvar o ZIP, a aplicacao calcula o SHA-256 e registra o processamento em `historical_imports`.

A importacao:

- inicia com status `processing`;
- termina com status `published` e a quantidade de registros;
- recebe status `failed` e a mensagem do erro em caso de falha;
- reconhece um arquivo publicado pelo mesmo SHA-256 e evita processa-lo novamente;
- limpa linhas parciais quando uma importacao falha;
- mantem a versao anual anterior publicada enquanto a nova versao esta em processamento;
- publica a nova versao em uma transacao, remove as cotacoes substituidas e marca a importacao anterior como `superseded`.

Cada cotacao guarda `import_id`, `line_number` e `record_sha256`, permitindo identificar o arquivo, a linha de origem e registros repetidos dentro da mesma importacao. A view `published_historical_quotes` expoe somente a versao anual publicada e deve ser usada por servicos de consulta.

### Gravacao em lote

As cotacoes sao enviadas ao PostgreSQL em blocos de 10.000 registros com `pgx.CopyFrom`, que utiliza o protocolo `COPY`.

Valores monetarios sao convertidos para `NUMERIC` com a escala correta. Por exemplo:

```text
Valor no arquivo: 12638
Valor no banco:   126.38
```

Os dados sao armazenados em:

- `historical_imports`: controle, auditoria e deduplicacao das importacoes;
- `historical_quotes`: cotacoes historicas extraidas dos registros tipo `01`.

### Geracao das queries com sqlc

Os comandos de controle da importacao (`SELECT`, `INSERT`, `UPDATE` e `DELETE`) ficam em:

```text
internal/adapters/outbound/postgres/queries/historical_imports.sql
```

O `sqlc.yaml` usa as migrations como schema e gera os tipos e metodos do adapter em `internal/adapters/outbound/postgres/sqlcgen`. Depois de alterar uma migration ou query, regenere o pacote:

```powershell
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate
```

O repository utiliza os metodos gerados pelo sqlc dentro das transacoes. A carga das cotacoes continua usando `pgx.CopyFrom`, pois o protocolo `COPY` e mais adequado para os lotes de 10.000 registros.

### Executar uma importacao

A partir da raiz do projeto:

```powershell
go run ./cmd 2026
```

Consulte o lote criado:

```sql
SELECT id, reference_year, status, total_records, completed_at, published_at
FROM historical_imports
ORDER BY id DESC;
```

Consulte as cotacoes de um ticker:

```sql
SELECT trading_date, ticker, open_price, high_price, low_price, close_price
FROM published_historical_quotes
WHERE ticker = 'PETR4'
ORDER BY trading_date DESC
LIMIT 20;
```
