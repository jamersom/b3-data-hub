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
9. O caso de uso conclui a importação e retorna o resultado do processamento.

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
|-- compose.yaml
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

O ambiente local utiliza PostgreSQL 17 e e configurado pelo arquivo `compose.yaml`.

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

O servico `importer-scheduler` permanece ativo com o `crond` e executa `/app/b3-data-hub` todos os dias as `00:00`, no fuso `America/Sao_Paulo`. O binario e construido pelo `Dockerfile`, e os arquivos baixados permanecem no volume local `./data`.

Acompanhe as importacoes:

```bash
docker compose logs -f importer-scheduler
```

Confira o horario e o agendamento dentro do container:

```bash
docker compose exec importer-scheduler date
docker compose exec importer-scheduler cat /etc/crontabs/root
```

Execute imediatamente para testar sem esperar a meia-noite:

```bash
docker compose exec importer-scheduler /app/b3-data-hub
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
- respeita cancelamento e timeout por `context.Context`.

O TXT descompactado e processado linha por linha. Ele nao e carregado por inteiro na memoria.

O download do ZIP também é feito em streaming para um arquivo temporário. Durante a cópia, a aplicação calcula o SHA-256 e rejeita respostas acima de 512 MiB. Depois da validação, o arquivo temporário é movido para `data`, evitando manter o ZIP completo na memória.

### Controle da importacao

Depois de validar e salvar o ZIP, a aplicacao calcula o SHA-256 e registra o processamento em `historical_imports`.

A importacao:

- inicia com status `processing`;
- termina com status `completed` e a quantidade de registros;
- recebe status `failed` e a mensagem do erro em caso de falha;
- reconhece um arquivo concluido pelo mesmo SHA-256 e evita processa-lo novamente;
- limpa linhas parciais e reinicia o lote quando o mesmo arquivo havia falhado anteriormente.

Cada cotacao guarda `import_id` e `line_number`, permitindo identificar o arquivo e a linha que originaram o registro.

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
SELECT id, reference_year, status, total_records, completed_at
FROM historical_imports
ORDER BY id DESC;
```

Consulte as cotacoes de um ticker:

```sql
SELECT trading_date, ticker, open_price, high_price, low_price, close_price
FROM historical_quotes
WHERE ticker = 'PETR4'
ORDER BY trading_date DESC
LIMIT 20;
```
