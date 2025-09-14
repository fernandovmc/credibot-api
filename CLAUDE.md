# Credibot API - Assistente Inteligente de Análise de Crédito

## 🎯 **CONTEXTO E OBJETIVO DO PROJETO**

Este projeto é uma **API de assistente inteligente especializado em análise de crédito** que combina inteligência artificial (OpenAI GPT) com consultas em tempo real a um banco de dados financeiro (Supabase). O objetivo é permitir que usuários façam perguntas em linguagem natural sobre dados de crédito e recebam respostas precisas baseadas em informações reais do banco.

### **Funcionamento Principal:**
1. **Usuário faz pergunta**: Ex: "Quais são os clientes com maior score de crédito?"
2. **IA analisa a pergunta** e determina se precisa consultar o banco de dados
3. **IA gera query SQL** automaticamente para buscar os dados necessários
4. **Sistema executa a query** no Supabase de forma segura (apenas SELECT)
5. **IA interpreta os resultados** e responde em linguagem natural com os dados encontrados

### **Vantagens desta Abordagem:**
- ✅ **Economia de tokens**: A IA não precisa conhecer todos os dados, apenas gera queries
- ✅ **Dados sempre atualizados**: Consultas em tempo real ao banco
- ✅ **Segurança**: Apenas consultas SELECT são permitidas (read-only)
- ✅ **Interface natural**: Usuários fazem perguntas como fariam para um analista humano

---

## 🏦 **DOMÍNIO DE NEGÓCIO: ANÁLISE DE CRÉDITO**

Este sistema é especializado em análise de crédito para instituições financeiras, cobrindo:

### **Principais Funcionalidades:**
- 📊 **Análise de Clientes**: Perfil de risco, score, histórico
- 💰 **Operações de Crédito**: Empréstimos ativos, valores, status
- 📈 **Histórico de Pagamentos**: Adimplência, atrasos, comportamento
- 🎯 **Modalidades de Crédito**: Tipos disponíveis, taxas, prazos
- 📋 **Análises Realizadas**: Decisões, aprovações, negações

### **Tipos de Perguntas que o Sistema Responde:**
- "Quais clientes têm score acima de 800?"
- "Mostre as operações em atraso há mais de 30 dias"
- "Qual a taxa média aprovada para empréstimos pessoais?"
- "Quantos clientes foram aprovados este mês?"
- "Liste os clientes com maior faturamento anual"

---

## 📊 **ESTRUTURA DO BANCO DE DADOS**

### **1. Tabela `clientes`** - Informações dos Clientes
**Propósito**: Armazena dados de clientes pessoa física (PF) e jurídica (PJ)

```sql
CREATE TABLE public.clientes (
  id integer PRIMARY KEY,
  uuid uuid DEFAULT uuid_generate_v4() UNIQUE,
  tipo_pessoa character varying CHECK (tipo_pessoa IN ('PF', 'PJ')),
  nome character varying NOT NULL,
  cpf_cnpj character varying NOT NULL UNIQUE,
  data_nascimento date,                    -- Para PF
  renda_mensal numeric,                    -- Para PF
  profissao character varying,             -- Para PF
  estado_civil character varying,          -- Para PF
  escolaridade character varying,          -- Para PF
  data_fundacao date,                      -- Para PJ
  faturamento_anual numeric,               -- Para PJ
  setor character varying,                 -- Para PJ
  porte character varying,                 -- Para PJ (Micro, Pequeno, Médio, Grande)
  cnae character varying,                  -- Para PJ
  uf character varying NOT NULL,
  cidade character varying,
  cep character varying,
  endereco text,
  telefone character varying,
  email character varying,
  score_credito integer CHECK (score_credito >= 0 AND score_credito <= 1000),
  classe_risco character varying DEFAULT 'AA',  -- AA, A, B, C, D, E
  created_at timestamp with time zone DEFAULT now(),
  updated_at timestamp with time zone DEFAULT now(),
  ativo boolean DEFAULT true
);
```

**Campos Importantes:**
- `tipo_pessoa`: 'PF' (Pessoa Física) ou 'PJ' (Pessoa Jurídica)
- `score_credito`: Valor entre 0 e 1000 (quanto maior, melhor)
- `classe_risco`: AA (menor risco) até E (maior risco)

---

### **2. Tabela `analises_credito`** - Análises Realizadas
**Propósito**: Registra todas as análises de crédito realizadas

```sql
CREATE TABLE public.analises_credito (
  id integer PRIMARY KEY,
  uuid uuid DEFAULT uuid_generate_v4() UNIQUE,
  cliente_id integer REFERENCES clientes(id),
  tipo_analise character varying NOT NULL,
  valor_solicitado numeric NOT NULL,
  modalidade_solicitada character varying NOT NULL,
  decisao character varying CHECK (decisao IN ('Aprovado', 'Negado', 'Pendente', 'Aprovado Condicional')),
  valor_aprovado numeric,
  taxa_aprovada numeric,
  prazo_aprovado integer,
  fatores_aprovacao jsonb,
  fatores_negacao jsonb,
  condicoes jsonb,
  score_calculado integer,
  probabilidade_inadimplencia numeric,
  modelo_usado character varying,
  analista_responsavel character varying,
  data_analise timestamp with time zone NOT NULL,
  observacoes text,
  created_at timestamp with time zone DEFAULT now()
);
```

**Campos Importantes:**
- `decisao`: 'Aprovado', 'Negado', 'Pendente', 'Aprovado Condicional'
- `valor_solicitado` vs `valor_aprovado`: Comparação para análise
- `fatores_aprovacao/negacao`: Detalhes em JSON
- `probabilidade_inadimplencia`: Percentual de risco

---

### **3. Tabela `operacoes_credito`** - Operações Ativas
**Propósito**: Controla empréstimos e financiamentos ativos

```sql
CREATE TABLE public.operacoes_credito (
  id integer PRIMARY KEY,
  uuid uuid DEFAULT uuid_generate_v4() UNIQUE,
  cliente_id integer REFERENCES clientes(id),
  modalidade character varying NOT NULL,      -- Tipo de crédito
  sub_modalidade character varying,
  valor_contratado numeric NOT NULL,
  valor_liberado numeric,
  valor_saldo numeric DEFAULT 0,             -- Saldo devedor atual
  data_contratacao date NOT NULL,
  data_primeiro_vencimento date,
  data_vencimento date,
  data_liquidacao date,
  prazo_meses integer,
  taxa_juros_mensal numeric,
  taxa_juros_anual numeric,
  indexador character varying,               -- CDI, IPCA, etc.
  forma_pagamento character varying,
  status character varying DEFAULT 'Ativo' CHECK (status IN ('Ativo', 'Liquidado', 'Vencido', 'Baixado')),
  classificacao_risco character varying DEFAULT 'AA',
  dias_atraso integer DEFAULT 0,
  tipo_garantia character varying,
  valor_garantia numeric,
  origem_recurso character varying,
  instituicao_origem character varying,
  agencia character varying,
  created_at timestamp with time zone DEFAULT now(),
  updated_at timestamp with time zone DEFAULT now()
);
```

**Campos Importantes:**
- `status`: 'Ativo', 'Liquidado', 'Vencido', 'Baixado'
- `dias_atraso`: Contador para controle de inadimplência
- `valor_saldo`: Valor ainda devido pelo cliente

---

### **4. Tabela `historico_pagamentos`** - Histórico de Pagamentos
**Propósito**: Registra todos os pagamentos realizados

```sql
CREATE TABLE public.historico_pagamentos (
  id integer PRIMARY KEY,
  uuid uuid DEFAULT uuid_generate_v4() UNIQUE,
  operacao_id integer REFERENCES operacoes_credito(id),
  numero_parcela integer,
  data_vencimento date NOT NULL,
  data_pagamento date,
  valor_parcela numeric NOT NULL,
  valor_pago numeric,
  valor_juros numeric,
  valor_principal numeric,
  dias_atraso integer DEFAULT 0,
  status character varying DEFAULT 'Pendente' CHECK (status IN ('Pendente', 'Pago', 'Atrasado', 'Renegociado')),
  created_at timestamp with time zone DEFAULT now()
);
```

**Campos Importantes:**
- `status`: 'Pendente', 'Pago', 'Atrasado', 'Renegociado'
- `dias_atraso`: Dias entre vencimento e pagamento
- `valor_pago` vs `valor_parcela`: Para identificar pagamentos parciais

---

### **5. Tabela `modalidades_credito`** - Tipos de Crédito
**Propósito**: Define produtos de crédito disponíveis

```sql
CREATE TABLE public.modalidades_credito (
  id integer PRIMARY KEY,
  uuid uuid DEFAULT uuid_generate_v4() UNIQUE,
  nome character varying NOT NULL UNIQUE,    -- Nome do produto
  categoria character varying NOT NULL,       -- Categoria (Pessoal, Empresarial, etc.)
  taxa_minima numeric,                       -- Taxa mínima oferecida
  taxa_maxima numeric,                       -- Taxa máxima oferecida
  prazo_minimo integer,                      -- Prazo mínimo em meses
  prazo_maximo integer,                      -- Prazo máximo em meses
  valor_minimo numeric,                      -- Valor mínimo de empréstimo
  valor_maximo numeric,                      -- Valor máximo de empréstimo
  ativo boolean DEFAULT true,
  created_at timestamp with time zone DEFAULT now(),
  updated_at timestamp with time zone DEFAULT now()
);
```

---

### **6. Tabela `score_historico`** - Evolução dos Scores
**Propósito**: Acompanha evolução do score de crédito dos clientes

```sql
CREATE TABLE public.score_historico (
  id integer PRIMARY KEY,
  uuid uuid DEFAULT uuid_generate_v4() UNIQUE,
  cliente_id integer REFERENCES clientes(id),
  score_anterior integer,
  score_atual integer NOT NULL,
  data_calculo date NOT NULL,
  fatores_impacto text[],                    -- Array de fatores
  score_pagamento integer,                   -- Componente: histórico de pagamentos
  score_historico integer,                   -- Componente: tempo de relacionamento
  score_utilizacao integer,                  -- Componente: utilização do crédito
  score_diversificacao integer,              -- Componente: diversificação
  score_tempo integer,                       -- Componente: tempo no mercado
  observacoes text,
  created_at timestamp with time zone DEFAULT now()
);
```

---

## 🔗 **RELACIONAMENTOS IMPORTANTES**

```
clientes (1) -----> (N) analises_credito
clientes (1) -----> (N) operacoes_credito  
clientes (1) -----> (N) score_historico
operacoes_credito (1) -----> (N) historico_pagamentos
```

---

## 🎯 **EXEMPLOS DE QUERIES TÍPICAS**

### **Clientes com Melhor Score:**
```sql
SELECT nome, cpf_cnpj, score_credito, classe_risco 
FROM clientes 
WHERE ativo = true 
ORDER BY score_credito DESC 
LIMIT 10;
```

### **Operações em Atraso:**
```sql
SELECT c.nome, o.modalidade, o.valor_saldo, o.dias_atraso
FROM operacoes_credito o
JOIN clientes c ON c.id = o.cliente_id
WHERE o.status = 'Vencido' AND o.dias_atraso > 30
ORDER BY o.dias_atraso DESC;
```

### **Análises Aprovadas no Mês:**
```sql
SELECT COUNT(*) as aprovados, AVG(valor_aprovado) as valor_medio
FROM analises_credito 
WHERE decisao = 'Aprovado' 
AND data_analise >= date_trunc('month', CURRENT_DATE);
```

---

## ⚠️ **REGRAS DE SEGURANÇA**

### **Para IA/Sistema:**
1. ✅ **APENAS queries SELECT** são permitidas
2. ❌ **NUNCA** usar INSERT, UPDATE, DELETE, DROP, ALTER
3. ✅ **SEMPRE** usar LIMIT para evitar sobrecarga
4. ✅ **Validar** queries antes da execução
5. ✅ **Tratar** campos sensíveis com cuidado

### **Para Usuários:**
- Sistema é **SOMENTE LEITURA** - não permite alterações no banco
- Dados sensíveis como CPF podem ser mascarados nas respostas
- Todas as consultas são logadas para auditoria

---

## 🤖 **INSTRUÇÕES PARA IA**

Quando receber perguntas sobre este sistema:

1. **Identifique** se a pergunta requer consulta ao banco
2. **Gere SQL válido** seguindo as regras de segurança
3. **Interprete os resultados** de forma clara para o usuário
4. **Formate números** adequadamente (R$ para valores, % para percentuais)
5. **Destaque** informações importantes
6. **Explique** o contexto quando necessário

**Exemplo de Fluxo:**
- Pergunta: "Quantos clientes PJ têm score acima de 700?"
- SQL: `SELECT COUNT(*) FROM clientes WHERE tipo_pessoa = 'PJ' AND score_credito > 700 AND ativo = true;`
- Resposta: "Encontrei 45 clientes pessoas jurídicas com score de crédito acima de 700 pontos."