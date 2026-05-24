# Configuração

Toda a configuração do Orqen vive em um único arquivo: `.orqen/orqen.yaml`.

## Estrutura do Arquivo

```
meu-projeto/
├── .orqen/
│   └── orqen.yaml       ← Configuração do workflow
└── ...
```

O arquivo tem três seções principais:

```yaml
agents:      # Quais agentes de IA estão disponíveis
execution:   # Como o motor executa
chat:        # Permite integraçao com o Telegram por exemplo
modules:     # Seus workflows (lanes + artefatos)
```

## 1. Agents - Configurando Agentes de IA

Defina quais agentes de IA estão disponíveis e como invocá-los:

```yaml
agents:
  default: "claude"                    # Agente padrão
  clients:
    qwen:
      command: ["qwen", "--yolo", "--acp"]
    claude:
      command: ["npx", "-y", "@agentclientprotocol/claude-agent-acp", "--dangerously-skip-permissions"]
```

| Campo | Descrição |
|-------|-----------|
| `default` | Nome do agente padrão (deve existir em `clients`) |
| `clients.<nome>.command` | Comando para invocar o agente (inclua flags de modo autônomo + ACP) |

<!-- > Veja a [página de Agentes](agentes.md) para detalhes de cada agente. -->

## 2. Execution - Controle de Execução

```yaml
execution:
  max_agents: 10                   # Máximo de agentes concorrentes (0 = ilimitado)
  sleep_interval_seconds: 60       # Segundos entre ciclos de trabalho
```

| Campo | Default | Descrição |
|-------|---------|-----------|
| `max_agents` | 0 (ilimitado) | Máximo de agentes rodando simultaneamente |
| `sleep_interval_seconds` | 60 | Intervalo entre varreduras de trabalho |

## 3. Modules - Definindo Workflows

Módulos agrupam lanes em um diretório dedicado.

### Exemplo Mínimo

```yaml
modules:
  - name: task
    dir: "tasks"
    lanes:
      - name: "inbox"
        purpose: "Ideias do usuário para criar tarefas"
        artifacts: ["TASK"]
        agent_behavior:
          - "Lê a ideia do inbox"
          - "Decompõe em tarefas executáveis"
          - "Cria tarefas na lane backlog"

      - name: "backlog"
        purpose: "Tarefas aguardando priorização"
        user_action: "prioritize"

      - name: "doing"
        purpose: "Tarefa em implementação"
        artifacts: ["SUMMARY", "FAIL"]
        agent_behavior:
          - "Lê a tarefa e especificações"
          - "Implementa conforme requisitos"
          - "Cria SUMMARY ao concluir"
```

### Atributos de Lane

| Atributo | Obrigatório | Descrição |
|----------|-------------|-----------|
| `name` | Sim | Nome da lane (diretório gerado como `NN_nome`) |
| `purpose` | Sim | Descrição do propósito (injetado no prompt) |
| `agent_behavior` | Não | Passos sequenciais do agente |
| `artifacts` | Não | Tipos de artefatos que o agente pode criar |
| `user_action` | Não | Ação esperada do usuário (ex: "approve", "review") |
| `critical_rules` | Não | Regras absolutas que nunca podem ser ignoradas |
| `ignore_if_exists` | Não | Pula lane se houver itens nessas outras lanes |
| `ignore_if_dependency` | Não | Pula item se depender de itens nessas lanes |
| `extra_prompt` | Não | Contexto adicional injetado no prompt |

### Regras de Ignorar (Ignore Rules)

As ignore rules controlam quando uma lane deve ser pulada:

```yaml
lanes:
  - name: "doing"
    ignore_if_exists: ["draft"]              # Não trabalhe aqui se há algo em draft
    ignore_if_dependency: ["inbox", "backlog"]  # Não trabalhe se depende de itens nessas lanes
```

| Regra | Significado |
|-------|-------------|
| `ignore_if_exists` | "Não trabalhe aqui se já tem algo em X" |
| `ignore_if_not_exists` | "Não trabalhe aqui a menos que X exista" |
| `ignore_if_dependency` | "Não trabalhe neste item se ele depende de X" |
| `ignore_if_attr` | "Não trabalhe neste item se seus atributos batem com a condição" |

Referências entre módulos usam `modulo.lane`:
```yaml
ignore_if_exists: ["adr.draft"]     # Lane draft do módulo adr
ignore_if_exists: ["file:*.md"]     # Pattern de arquivo glob
```

### Ordem de Execução

O campo `order` controla a prioridade de verificação:

```yaml
modules:
  - name: task
    order: ["doing", "review", "inbox"]
```

**Raciocínio:** Verificar `doing` primeiro retoma trabalho em progresso. Depois `review` valida trabalho completo. Por último `inbox` pega trabalho novo.

## 4. Hooks - Scripts Pré e Pós Execução

Hooks permitem executar scripts automaticamente antes (pre) e depois (post) da execução de uma tarefa em uma lane.

### Definindo Hooks

Hooks são definidos com nome e podem ter variantes por sistema operacional:

```yaml
hooks:
  notify: ["echo", "Iniciando $WI"]
  notify.windows: ["cmd", "/c", "echo", "Iniciando %WI%"]
  setup: ["./scripts/setup.sh", "--item", "$WI", "--seq", "$ITEM_SEQ"]
```

| Campo | Descrição |
|-------|-----------|
| `hook_name` | Comando padrão (Unix/macOS/Linux) |
| `hook_name.windows` | Comando alternativo para Windows |
| `hook_name.darwin` | Comando alternativo para macOS |
| `hook_name.linux` | Comando alternativo para Linux |

> **Nota:** O sistema seleciona automaticamente a variante do SO. Se não houver variante para o SO atual, usa o comando padrão.

### Vinculando Hooks a Módulos e Lanes

Hooks podem ser vinculados no nível do módulo (afeta todas as lanes) ou no nível da lane (substitui ou estende os do módulo):

```yaml
modules:
  - name: task
    dir: "tasks"
    hooks:
      pre:
        - notify        # Executa antes de cada tarefa em qualquer lane
      post:
        - setup         # Executa após cada tarefa em qualquer lane
    lanes:
      - name: "inbox"
        purpose: "Ideias do usuário"
        hooks:
          post:
            - notify    # Hook específico apenas para esta lane
```

### Sintaxe de Exclusão (`!hook_name`)

Lanes podem excluir hooks herdados do módulo usando `!` antes do nome. **Atenção:** o valor deve estar entre aspas para o YAML não interpretar `!` como tag:

```yaml
lanes:
  - name: "review"
    hooks:
      post:
        - "!notify"     # NÃO executa notify nesta lane
        - setup         # Mas ainda executa setup
```

| Sintaxe | Significado |
|---------|-------------|
| `- notify` | Executa o hook `notify` |
| `- "!notify"` | **Remove** o hook `notify` herdado do módulo |

### Wildcards em Hooks

Dentro dos comandos de hook, as seguintes variáveis são expandidas automaticamente:

| Wildcard | Descrição | Exemplo |
|----------|-----------|---------|
| `$WI` | Caminho relativo do work item | `03_backlog/WI-0001-minha-tarefa` |
| `$MODULE` | Nome do módulo | `task` |
| `$LANE` | Nome da lane | `backlog` |
| `$ITEM_SEQ` | Número sequencial | `0001` |
| `$PROJECT_DIR` | Diretório raiz do projeto | `/home/user/meu-projeto` |
| `$WI_JSON` | Work item como string JSON | `{"id":"WI-0001",...}` |

Você pode usar `$VAR` ou `${VAR}` (útil para evitar ambiguidade, como `${WI}_extra`).

### Comportamento de Execução

| Tipo | Quando executa | Se falha |
|------|----------------|----------|
| **Pre-hook** | Antes da invocação do agente | Aborta a tarefa e cria artefato FAIL |
| **Post-hook** | Após a invocação do agente | Registra aviso e continua normalmente |

### Timeout

O tempo máximo de execução de um hook é configurado em `execution.hook_timeout_seconds` (padrão: 300 segundos = 5 minutos). Hooks que excedem este limite são interrompidos.

```yaml
execution:
  hook_timeout_seconds: 600    # 10 minutos
```

→ Para referência completa de todos os atributos, consulte [Referência Completa de Configuração](https://github.com/nidorx/orqen/blob/main/docs/CONFIG.md).

## Exemplo Completo Simplificado

```yaml
agents:
  default: "claude"
  clients:
    qwen:
      command: ["qwen", "--yolo", "--acp"]
    claude:
      command: ["npx", "-y", "@agentclientprotocol/claude-agent-acp", "--dangerously-skip-permissions"]

execution:
  max_agents: 10
  sleep_interval_seconds: 60
  hook_timeout_seconds: 300

hooks:
  notify: ["echo", "Processando $WI na lane $LANE"]
  notify.windows: ["cmd", "/c", "echo", "Processando %WI% na lane %LANE%"]

modules:
  - name: task
    dir: "tasks"
    order: ["doing", "review", "inbox"]
    hooks:
      pre:
        - notify
    lanes:
      - name: "inbox"
        purpose: "Ideias prontas para virar tarefas"
        artifacts: ["TASK"]
        agent_behavior:
          - "Lê a ideia do inbox"
          - "Decompõe em tarefas executáveis"
          - "Cria tarefas na lane backlog"
        critical_rules:
          - "Crie TODAS as tarefas do inbox em uma única invocação"

      - name: "doing"
        purpose: "Tarefa em implementação"
        artifacts: ["SUMMARY", "FAIL"]
        ignore_if_exists: ["draft"]
        agent_behavior:
          - "Lê a tarefa e implementa"
          - "Cria SUMMARY ao concluir ou FAIL se bloqueado"

      - name: "review"
        purpose: "Implementação aguardando revisão"
        hooks:
          pre:
            - "!notify"
        agent_behavior:
          - "Valida critérios de aceitação"
          - "Revisa qualidade do código"

      - name: "done"
        purpose: "Tarefas concluídas"
        user_action: "archive"
```

## Referência Completa

Para documentação completa de todos os atributos, incluindo o Condition Language para `ignore_if_attr`, consulte:

→ [Referência Completa de Configuração](https://github.com/nidorx/orqen/blob/main/docs/CONFIG.md)

## Próximos passos

- [Agentes](agentes.md) - Configure Qwen, Claude ou Copilot
- [Telegram](telegram.md) - Integre com bot do Telegram
- [Exemplos](exemplos.md) - Pipelines prontos para usar