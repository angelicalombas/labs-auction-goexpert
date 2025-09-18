# Desafio Fechamento automático do leilão
Objetivo: Adicionar uma nova funcionalidade ao projeto já existente para o leilão fechar automaticamente a partir de um tempo definido.

### Como rodar o projeto em ambiente de desenvolvimento
Pré-requisitos:

- Docker e Docker Compose instalados

- Git instalado

Clone e configuração do projeto:

```bash
git clone https://github.com/angelicalombas/labs-auction-goexpert
cd labs-auction-goexpert
```

### Execução com Docker Compose:

```bash
docker-compose up -d
```

### Verificação dos serviços:

Aplicação: http://localhost:8080

MongoDB Express: http://localhost:8081

MongoDB: localhost:27017

### Execução de testes:


- Executar testes:
```bash
go test ./internal/infra/database/auction/... -v
```

### Parar os serviços:

```bash
docker-compose down
```

### Funcionalidades implementadas:
- Cálculo de tempo do leilão: Função getAuctionDuration() que lê a variável de ambiente AUCTION_DURATION_MINUTES

- Goroutine de verificação: startAuctionChecker() que verifica leilões vencidos a cada 30 segundos

- Fechamento automático: checkAndCloseExpiredAuctions() que fecha leilões com tempo esgotado

- Controle de concorrência: Uso de sync.Mutex para evitar race conditions

- Testes automatizados: Testes para validar o fechamento automático e cálculo de duração
