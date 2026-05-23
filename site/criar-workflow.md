# Criar Workflow Customizado

Não encontrou um exemplo que atende sua necessidade? Use o skill **Orqen Workflow Creator** para criar um workflow customizado através de uma conversa guiada.

## O que é

O `orqen-workflow-create` é um skill embutido que entrevista você sobre suas necessidades de workflow e gera:

1. Um arquivo `.orqen/orqen.yaml` completo e válido
2. Templates de artefatos em `.orqen/{modulo}/prompts/`
3. Validação da configuração

## Como funciona

O processo é **conversacional** - uma ou duas perguntas por vez, sem dump de informações. O skill propõe, você refina, até estar satisfeito.

### Fases do Processo

```
FASE 1: Entender
  ↓
FASE 2: Propor (diagrama visual, NÃO YAML)
  ↓
FASE 3: Refinar (feedback do usuário)
  ↓
FASE 4: Gerar (cria orqen.yaml + templates)
  ↓
FASE 5: Verificar (valida configuração)
  ↓
FASE 6: Testar (sugere criar itens iniciais)
```

## O que o skill pergunta

O skill trabalha através de 8 tópicos, pulando os que já foram cobertos naturalmente:

| # | Tópico | O que é definido |
|---|--------|-----------------|
| 1 | **Visão do Projeto** | Para que é o workflow? Quem está envolvido? |
| 2 | **Estágios (Lanes)** | Quais são as etapas do início ao fim? Quem atua em cada uma? |
| 3 | **Artefatos** | Que arquivos o agente produz? Quais templates precisa? |
| 4 | **Regras e Constraints** | Regras absolutas que nunca podem ser violadas? |
| 5 | **Dependências e Bloqueios** | Uma etapa depende de outra? Lanes que bloqueiam outras? |
| 6 | **Configuração do Agente** | Qual agente? Flags de modo autônomo? Máximo concorrente? |
| 7 | **Ordem de Execução** | Quais lanes verificar primeiro? (work-in-progress antes de novo trabalho) |
| 8 | **Contexto Extra** | Conhecimento adicional que o agente deve carregar? |

## Exemplo de Interação

**Você:** "Quero um workflow para criar vídeos de YouTube"

**Skill:** "Entendi. Do início ao fim, quais são as etapas? Por exemplo: ideia → roteiro → gravação → edição → revisão → publicação. Faz sentido para você?"

**Você:** "Sim, mas quero aprovar o roteiro antes de gravar"

**Skill:** "Perfeito. Então teríamos um checkpoint humano aí. E quem faz a gravação e edição - é o agente ou você?"

... e assim por diante, até o workflow estar completo.

## O que é gerado

### 1. `.orqen/orqen.yaml`

```yaml
agents:
  default: "claude"
  clients:
    qwen:
      command: ["qwen", "--yolo", "--acp"]
    claude:
      command: ["npx", "-y", "@agentclientprotocol/claude-agent-acp", "--dangerously-skip-permissions"]

execution:
  max_agents: 5
  sleep_interval_seconds: 30

modules:
  - name: task
    dir: ".orqen/tasks"
    order: ["doing", "review", "inbox"]
    lanes:
      - name: "inbox"
        purpose: "Ideias de vídeo prontas para análise"
        # ... etc
```

### 2. Templates de Artefatos

Para cada tipo de artefato definido, o skill cria um template:

```
.orqen/
├── orqen.yaml
└── tasks/
    └── prompts/
        ├── ROTEIRO.md
        ├── CHECKLIST.md
        └── SUMMARY.md
```

Cada template tem seções claras com placeholders para o agente preencher.

### 3. Representação Visual (antes de gerar YAML)

O skill descreve o workflow com diagramas de texto simples:

```
Proposed Workflow:

  ┌─────────────┐
  │   INBOX     │  ← Ideias de vídeo
  │   (agent)   │  Agente analisa e cria ROTEIRO
  └──────┬──────┘
         │
         ▼
  ┌─────────────┐
  │ REVIEW      │  ← Humano aprova roteiro
  │ SCRIPT      │
  │   (user)    │
  └──────┬──────┘
         │
         ▼
  ┌─────────────┐
  │  PRODUCE    │  ← Agente gera vídeo
  │   (agent)   │
  └─────────────┘
```

## Regras Importantes

- **Nunca mostra YAML cru durante a entrevista** - o usuário entende o processo, não a sintaxe
- **Artefatos SEM extensão no config** - `["ROTEIRO", "CHECKLIST"]` e NÃO `["ROTEIRO.md"]`
- **Confirma antes de gerar arquivos** - o usuário aprova o design antes de qualquer criação
- **Templates em pt-br** se o workflow for em português

## Como invocar o skill

Se estiver usando um agente compatível com skills:

```
/orqen-workflow-create
```

Ou simplesmente descreva o que precisa: "Quero criar um workflow para ..."

## Dicas de Design

### Ordem de Execução

Recomendação geral:
1. Lanes que **retomam trabalho em progresso** primeiro (`doing`, `learning`)
2. Lanes de **review/aprovação** depois (`review`, `revisao`)
3. Lanes de **intake/novo trabalho** por último (`inbox`)

### Checkpoints Humanos

Use `user_action` para lanes onde o humano decide:
```yaml
- name: "review"
  purpose: "Conteúdo aguardando aprovação"
  user_action: "approve"
```

### Regras Críticas

Use `critical_rules` para regras que NUNCA podem ser ignoradas:
```yaml
critical_rules:
  - "Todo conteúdo deve ser em português brasileiro"
  - "Nunca inventar referências - devem ser reais e verificáveis"
```

## Próximos passos

- [Exemplos](exemplos.md) - Pipelines prontos para usar
- [Configuração](configuracao.md) - Referência de configuração
- [Conceitos](conceitos.md) - Entenda lanes, módulos e artefatos
