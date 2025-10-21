# Credibot API Documentation

**Base URL (Production):** `${API_PRODUCTION_URL}/api/v1`
**Base URL (Local):** `http://localhost:3000/api/v1`

> **Note:** Replace `${API_PRODUCTION_URL}` with your actual production URL from environment variables.

## Swagger UI

Access interactive API documentation at:
- **Production:** `${API_PRODUCTION_URL}/swagger/index.html`
- **Local:** `http://localhost:3000/swagger/index.html`

## Endpoints

### 1. Health Check

**GET** `/`

Verifica se a API está funcionando.

**Response:**
```json
{
  "message": "Credibot API running"
}
```

---

### 2. Chat Simples

**POST** `/api/v1/chat`

Envia uma mensagem para o OpenAI sem integração com banco de dados.

**Request Body:**
```json
{
  "message": "Como funciona análise de crédito?",
  "model": "gpt-3.5-turbo",
  "max_tokens": 150
}
```

**Parameters:**
- `message` (string, required): Mensagem do usuário
- `model` (string, optional): Modelo do OpenAI (default: gpt-3.5-turbo)
- `max_tokens` (int, optional): Número máximo de tokens (default: 150)

**Response:**
```json
{
  "success": true,
  "data": {
    "message": "Análise de crédito é o processo...",
    "model": "gpt-3.5-turbo",
    "usage": {
      "prompt_tokens": 20,
      "completion_tokens": 100,
      "total_tokens": 120
    },
    "created_at": "2025-10-21T13:30:00Z"
  },
  "message": "Chat response generated successfully"
}
```

---

### 3. Smart Chat (com Banco de Dados)

**POST** `/api/v1/smart-chat`

Envia uma mensagem inteligente que pode consultar o banco de dados automaticamente.

**Request Body:**
```json
{
  "message": "Quais clientes têm score acima de 800?"
}
```

**Parameters:**
- `message` (string, required): Pergunta do usuário

**Response:**
```json
{
  "success": true,
  "data": {
    "message": "Encontrei 15 clientes com score acima de 800...",
    "used_database": true,
    "sql_query": "SELECT nome, score_credito FROM clientes WHERE score_credito > 800",
    "created_at": "2025-10-21T13:30:00Z"
  },
  "message": "Smart chat response generated successfully"
}
```

---

### 4. Listar Clientes (Paginado com Filtros)

**GET** `/api/v1/clientes`

Lista clientes com paginação, campos otimizados e filtros avançados.

**Query Parameters:**

**Paginação:**
- `page` (int, optional): Número da página (default: 1)
- `per_page` (int, optional): Itens por página (default: 25, max: 100)

**Filtros:**
- `search` (string, optional): Busca por nome (case-insensitive, partial match)
- `score_min` (int, optional): Score mínimo (0-1000)
- `score_max` (int, optional): Score máximo (0-1000)
- `classe_risco` (string, optional): Filtro por classe de risco (ex: "Baixo", "Médio", "Alto")
- `tipo_pessoa` (string, optional): Filtro por tipo de pessoa ("PF" ou "PJ")
- `ativo` (boolean, optional): Filtro por status ativo (true/false)

**Examples:**

Básico com paginação:
```
GET /api/v1/clientes?page=1&per_page=25
```

Buscar por nome:
```
GET /api/v1/clientes?search=silva&page=1&per_page=25
```

Filtrar por score:
```
GET /api/v1/clientes?score_min=700&score_max=900&page=1
```

Filtrar por tipo e status:
```
GET /api/v1/clientes?tipo_pessoa=PF&ativo=true&page=1
```

Filtros combinados:
```
GET /api/v1/clientes?search=joão&score_min=750&classe_risco=Baixo&tipo_pessoa=PF&ativo=true
```

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "nome": "João Silva",
      "cpf_cnpj": "123.456.789-00",
      "score_credito": 750,
      "classe_risco": "Baixo",
      "tipo_pessoa": "PF",
      "ativo": true
    }
  ],
  "pagination": {
    "page": 1,
    "per_page": 25,
    "total": 150,
    "total_pages": 6
  },
  "message": "Clientes retrieved successfully"
}
```

**Campos Retornados:**
- `id`: UUID do cliente
- `nome`: Nome completo
- `cpf_cnpj`: CPF ou CNPJ formatado
- `score_credito`: Score de crédito (0-1000)
- `classe_risco`: Classificação de risco
- `tipo_pessoa`: PF (Pessoa Física) ou PJ (Pessoa Jurídica)
- `ativo`: Status ativo/inativo

---

### 5. Buscar Cliente por ID

**GET** `/api/v1/cliente/:id`

Busca um cliente específico com todos os dados completos.

**Parameters:**
- `id` (string, required): UUID do cliente

**Example:**
```
GET /api/v1/cliente/550e8400-e29b-41d4-a716-446655440000
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "nome": "João Silva",
    "cpf_cnpj": "123.456.789-00",
    "score_credito": 750,
    "classe_risco": "Baixo",
    "tipo_pessoa": "PF",
    "renda_mensal": 5000.00,
    "faturamento_anual": null,
    "uf": "SP",
    "cidade": "São Paulo",
    "ativo": true,
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2025-10-20T15:45:00Z"
  },
  "message": "Cliente retrieved successfully"
}
```

**Error Response (404):**
```json
{
  "error": true,
  "message": "Cliente not found",
  "code": 404
}
```

---

### 6. Buscar Dados de Outras Tabelas (Genérico)

**GET** `/api/v1/data/:table`

Endpoint genérico para buscar dados de outras tabelas do banco.

**Parameters:**
- `table` (string, required): Nome da tabela (ex: analises_credito, operacoes_credito)

**Query Parameters:**
- `limit` (int, optional): Número de registros (default: 10, max: 50)
- `offset` (int, optional): Offset para paginação (default: 0)
- `order_by` (string, optional): Campo para ordenação (default: created_at)

**Example:**
```
GET /api/v1/data/analises_credito?limit=20&offset=0&order_by=data_analise
```

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "...",
      "cliente_id": "...",
      "decisao": "Aprovado",
      "valor_solicitado": 10000.00,
      "valor_aprovado": 8000.00,
      "taxa_aprovada": 2.5,
      "data_analise": "2025-10-15T10:00:00Z",
      "modalidade_solicitada": "Crédito Pessoal"
    }
  ],
  "message": "Data retrieved successfully"
}
```

---

## Tabelas Disponíveis

### clientes
Informações dos clientes (PF e PJ)

### analises_credito
Histórico de análises de crédito realizadas

### operacoes_credito
Operações de crédito contratadas

### historico_pagamentos
Histórico de pagamentos das operações

### modalidades_credito
Modalidades de crédito disponíveis

---

## Error Responses

Todos os endpoints podem retornar os seguintes erros:

**400 Bad Request:**
```json
{
  "error": true,
  "message": "Invalid request format",
  "code": 400
}
```

**404 Not Found:**
```json
{
  "error": true,
  "message": "Cliente not found",
  "code": 404
}
```

**500 Internal Server Error:**
```json
{
  "error": true,
  "message": "Failed to fetch data: ...",
  "code": 500
}
```

---

## Headers Requeridos

Para requisições POST:
```
Content-Type: application/json
```

---

## Boas Práticas

1. **Paginação**: Sempre use paginação para listas grandes
   - Use `per_page` adequado (recomendado: 25-50)
   - Implemente infinite scroll ou botões de página

2. **Cache**: Considere cachear listas de clientes no frontend
   - TTL recomendado: 5-10 minutos

3. **Loading States**: Implemente estados de loading nas requisições

4. **Error Handling**: Sempre trate erros 4xx e 5xx adequadamente

5. **Detalhes do Cliente**: Use `/cliente/:id` apenas quando necessário
   - Evite buscar todos os dados se não precisar

6. **Smart Chat**: Use para perguntas complexas sobre dados
   - Exemplos: "Clientes inadimplentes", "Maiores operações", etc.

---

## Exemplos de Uso (JavaScript/TypeScript)

### Setup Base URL
```javascript
const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:3000/api/v1';
```

### Listar Clientes com Paginação
```javascript
async function getClientes(page = 1, perPage = 25) {
  const response = await fetch(
    `${API_BASE_URL}/clientes?page=${page}&per_page=${perPage}`
  );
  const data = await response.json();
  return data;
}
```

### Listar Clientes com Filtros
```javascript
async function getClientesWithFilters(filters = {}) {
  const params = new URLSearchParams({
    page: filters.page || 1,
    per_page: filters.perPage || 25,
    ...(filters.search && { search: filters.search }),
    ...(filters.scoreMin && { score_min: filters.scoreMin }),
    ...(filters.scoreMax && { score_max: filters.scoreMax }),
    ...(filters.classeRisco && { classe_risco: filters.classeRisco }),
    ...(filters.tipoPessoa && { tipo_pessoa: filters.tipoPessoa }),
    ...(filters.ativo !== undefined && { ativo: filters.ativo }),
  });

  const response = await fetch(`${API_BASE_URL}/clientes?${params}`);
  const data = await response.json();
  return data;
}

// Exemplo de uso:
const clientes = await getClientesWithFilters({
  search: 'silva',
  scoreMin: 700,
  tipoPessoa: 'PF',
  ativo: true,
  page: 1,
  perPage: 25
});
```

### Buscar Cliente por ID
```javascript
async function getClienteById(id) {
  const response = await fetch(`${API_BASE_URL}/cliente/${id}`);
  if (!response.ok) {
    throw new Error('Cliente not found');
  }
  const data = await response.json();
  return data.data;
}
```

### Smart Chat
```javascript
async function smartChat(message) {
  const response = await fetch(`${API_BASE_URL}/smart-chat`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ message }),
  });
  const data = await response.json();
  return data.data;
}
```

---

## Environment Variables

### Required
```env
PORT=3000
API_PRODUCTION_URL=https://your-api-url.railway.app
SUPABASE_URL=your_supabase_url
SUPABASE_API_KEY=your_supabase_key
OPENAI_API_KEY=your_openai_key
```

### Optional
```env
OPENAI_MODEL=gpt-3.5-turbo
OPENAI_MAX_TOKENS=500
OPENAI_TEMPERATURE=0.7
```

---

## Notas Importantes

- **HTTPS Obrigatório em Produção**: Use sempre `https://` em produção
- **Rate Limiting**: Sem limite atual, mas use com moderação
- **Timeout**: Requisições podem levar até 30s (especialmente smart-chat)
- **Swagger UI**: Acesse `/swagger/index.html` para documentação interativa
