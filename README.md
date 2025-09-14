# Credibot API

Uma API REST construída com Go Fiber que integra OpenAI e Supabase para funcionalidades de chat e gerenciamento de dados.

## 🚀 Características

- **Framework**: Go Fiber (alta performance)
- **IA**: Integração com OpenAI GPT
- **Banco de Dados**: Supabase (PostgreSQL)
- **Arquitetura**: Clean Architecture com separação de responsabilidades
- **Tratamento de Erros**: Sistema robusto de error handling
- **Configuração**: Gerenciamento via variáveis de ambiente

## 📋 Pré-requisitos

- Go 1.21 ou superior
- Conta OpenAI com API Key
- Projeto Supabase configurado

## 🛠️ Instalação

1. **Clone o repositório**
```bash
git clone <seu-repositorio>
cd credibot-api
```

2. **Instale as dependências**
```bash
go mod tidy
```

3. **Configure as variáveis de ambiente**
```bash
cp .env.example .env
```

4. **Edite o arquivo `.env` com suas credenciais**
```env
# Server Configuration
PORT=3000

# Supabase Configuration
SUPABASE_URL=https://your-project.supabase.co
SUPABASE_API_KEY=your_supabase_anon_key_here

# OpenAI Configuration
OPENAI_API_KEY=sk-your_openai_api_key_here
OPENAI_MODEL=gpt-3.5-turbo
OPENAI_MAX_TOKENS=150
OPENAI_TEMPERATURE=0.7
```

5. **Execute a aplicação**
```bash
go run .
```

Ou compile e execute:
```bash
go build -o credibot-api .
./credibot-api
```

## 📚 Estrutura do Projeto

```
credibot-api/
├── main.go              # Servidor principal
├── config/
│   └── config.go        # Configurações da aplicação
├── handlers/
│   ├── chat.go          # Handlers do OpenAI
│   └── supabase.go      # Handlers do Supabase
├── models/
│   └── types.go         # Tipos e structs
├── go.mod               # Dependências
├── go.sum               # Lock file
├── .env.example         # Template de configuração
└── README.md            # Este arquivo
```

## 🔗 API Endpoints

### Health Check

#### `GET /`
Verifica se a API está funcionando.

**Resposta:**
```json
{
  "message": "Credibot API running"
}
```

---

### Chat com OpenAI

#### `POST /api/v1/chat`
Chat básico com OpenAI (sem integração ao banco de dados).

**Body da Requisição:**
```json
{
  "message": "O que é análise de crédito?",
  "model": "gpt-3.5-turbo",        // Opcional
  "max_tokens": 150                // Opcional
}
```

**Resposta de Sucesso (200):**
```json
{
  "success": true,
  "data": {
    "message": "Análise de crédito é o processo...",
    "model": "gpt-3.5-turbo",
    "usage": {
      "prompt_tokens": 12,
      "completion_tokens": 45,
      "total_tokens": 57
    },
    "created_at": "2024-01-15T10:30:00Z"
  },
  "message": "Chat response generated successfully"
}
```

#### `POST /api/v1/smart-chat`
Chat inteligente com integração ao banco de dados. **Esta é a funcionalidade principal do Credibot!**

A IA analisa sua pergunta, determina se precisa consultar o banco, gera SQL automaticamente e responde com dados reais.

**Body da Requisição:**
```json
{
  "message": "Quais são os clientes com maior score de crédito?"
}
```

**Resposta de Sucesso (200):**
```json
{
  "success": true,
  "data": {
    "message": "Encontrei os clientes com maior score de crédito:\n\n1. **João Silva** - Score: 950 (Classe AA)\n2. **Maria Santos** - Score: 920 (Classe AA)\n3. **Pedro Costa** - Score: 890 (Classe AA)\n\nTodos estão na classificação de menor risco (AA) e são excelentes candidatos para novas operações de crédito.",
    "used_database": true,
    "sql_query": "SELECT nome, score_credito, classe_risco FROM clientes WHERE ativo = true ORDER BY score_credito DESC LIMIT 10",
    "created_at": "2024-01-15T10:30:00Z"
  },
  "message": "Smart chat response generated successfully"
}
```

**Exemplos de Perguntas:**
- "Quantos clientes PJ têm score acima de 800?"
- "Mostre as operações em atraso há mais de 30 dias"
- "Qual é a taxa média aprovada para empréstimos pessoais?"
- "Liste os clientes com maior faturamento anual"
- "Quantas análises foram aprovadas este mês?"

---

### Consulta Direta aos Dados (Somente Leitura)

#### `GET /api/v1/data/:table`
Busca dados diretamente de uma tabela específica. **Disponível apenas para consultas diretas - para análises inteligentes, use `/smart-chat`**.

**Parâmetros de Query:**
- `limit`: Número máximo de registros (padrão: 10)
- `offset`: Número de registros para pular (padrão: 0)
- `order_by`: Campo para ordenação (padrão: created_at)

**Tabelas Disponíveis:**
- `clientes` - Informações dos clientes
- `analises_credito` - Análises de crédito realizadas
- `operacoes_credito` - Operações de crédito ativas
- `historico_pagamentos` - Histórico de pagamentos
- `modalidades_credito` - Modalidades de crédito disponíveis
- `score_historico` - Histórico de scores

**Exemplo:**
```
GET /api/v1/data/clientes?limit=5&order_by=score_credito
```

**Resposta de Sucesso (200):**
```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "nome": "João Silva",
      "cpf_cnpj": "***.***.***-**",
      "score_credito": 850,
      "classe_risco": "AA",
      "created_at": "2024-01-15T10:30:00Z"
    }
  ],
  "message": "Data retrieved successfully"
}
```

**⚠️ Nota de Segurança:** 
- Apenas operações de **leitura (GET)** são permitidas
- Dados sensíveis como CPF podem aparecer mascarados
- Para análises complexas, prefira usar `/smart-chat`

---

## 🔧 Configuração

### Variáveis de Ambiente

| Variável | Descrição | Padrão |
|----------|-----------|---------|
| `PORT` | Porta do servidor | `3000` |
| `SUPABASE_URL` | URL do projeto Supabase | - |
| `SUPABASE_API_KEY` | Chave da API do Supabase | - |
| `OPENAI_API_KEY` | Chave da API do OpenAI | - |
| `OPENAI_MODEL` | Modelo do OpenAI a usar | `gpt-3.5-turbo` |
| `OPENAI_MAX_TOKENS` | Limite de tokens por resposta | `150` |
| `OPENAI_TEMPERATURE` | Criatividade das respostas (0-1) | `0.7` |

### Configuração do Supabase

1. Crie um projeto no [Supabase](https://supabase.com/)
2. Obtenha a URL do projeto e a chave anônima
3. Configure suas tabelas no banco de dados
4. Defina as políticas RLS (Row Level Security) se necessário

### Configuração do OpenAI

1. Crie uma conta na [OpenAI](https://openai.com/)
2. Gere uma API Key no dashboard
3. Configure os limites de uso conforme necessário

---

## 📝 Exemplos de Uso

### Exemplo com cURL

**Smart Chat (Funcionalidade Principal):**
```bash
curl -X POST http://localhost:3000/api/v1/smart-chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Quais são os clientes com maior score de crédito?"
  }'
```

**Chat Básico:**
```bash
curl -X POST http://localhost:3000/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "O que é análise de crédito?",
    "max_tokens": 100
  }'
```

**Consulta Direta:**
```bash
curl http://localhost:3000/api/v1/data/clientes?limit=5&order_by=score_credito
```

### Exemplo com JavaScript/Fetch

```javascript
// Smart Chat - Funcionalidade Principal
const smartChatResponse = await fetch('http://localhost:3000/api/v1/smart-chat', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({
    message: 'Quantos clientes PJ têm score acima de 800?'
  })
});

const smartData = await smartChatResponse.json();
console.log('Resposta:', smartData.data.message);
console.log('Usou BD:', smartData.data.used_database);
console.log('SQL:', smartData.data.sql_query);

// Chat Básico
const chatResponse = await fetch('http://localhost:3000/api/v1/chat', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({
    message: 'O que é análise de crédito?'
  })
});

// Consulta Direta
const clientsResponse = await fetch('http://localhost:3000/api/v1/data/clientes?limit=10');
const clients = await clientsResponse.json();
console.log(clients.data);
```

---

## 🛡️ Tratamento de Erros

A API utiliza códigos de status HTTP padrão e retorna erros no formato:

```json
{
  "error": true,
  "message": "Descrição do erro",
  "code": 400
}
```

### Códigos de Status Comuns

- `200`: Sucesso
- `201`: Criado com sucesso
- `400`: Erro na requisição (dados inválidos)
- `500`: Erro interno do servidor

---

## 🔍 Logs e Monitoramento

A aplicação inclui:
- Log de requisições HTTP (via Fiber Logger middleware)
- Validação de configurações no startup
- Tratamento centralizado de erros
- Headers CORS configurados

---

## 🚀 Deploy

### Docker (Opcional)

Crie um `Dockerfile`:
```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o credibot-api .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/credibot-api .
COPY --from=builder /app/.env .

CMD ["./credibot-api"]
```

### Variáveis de Ambiente em Produção

Certifique-se de definir todas as variáveis necessárias no ambiente de produção:

```bash
export PORT=8080
export SUPABASE_URL=https://your-project.supabase.co
export SUPABASE_API_KEY=your_production_key
export OPENAI_API_KEY=your_production_openai_key
```

---

**Desenvolvido com ❤️ usando Go Fiber, OpenAI e Supabase**