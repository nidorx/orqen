# Exemplo: Dev Assistido

Desenvolvimento com **checkpoints humanos**. O agente faz o trabalho pesado, mas o humano decide nos pontos críticos.

## Pipeline

```
┌─────────┐    ┌────────────────┐    ┌────────────────┐    ┌─────────────────────┐    ┌────────────────┐
│  INBOX  │───>│ REVISAR ISSUE  │───>│ ESPECIFICACAO  │───>│ REVISAR ESPECIFICAC │───>│ IMPLEMENTACAO  │
│ (agent) │    │   (humano)     │    │   (agent)      │    │     (humano)        │    │   (agent)      │
└─────────┘    └────────────────┘    └────────────────┘    └─────────────────────┘    └───────┬────────┘
                                                                                              │
                                                              ┌───────────────────────────────┘
                                                              ▼
┌──────────────┐    ┌───────────┐    ┌───────────────┐    ┌──────────────────────────────────────┐
│  CONCLUIDO   │<───│ PR ABERTO │<───│   REVISAO     │<───│              BLOCKED                 │
│  (arquivar)  │    │  (agent)  │    │   (agent)     │    │     (humano resolve bloqueios)       │
└──────────────┘    └───────────┘    └───────────────┘    └──────────────────────────────────────┘
```

**Checkpoints humanos:** Revisar Issue → Revisar Especificação → Merge

## Configuração

<details>
<summary>Clique para ver o orqen.yaml completo</summary>

```yaml
# Orqen Workflow — Desenvolvimento Assistido com Checkpoint Humano
# Pipeline: Issue → Especificação → Implementação → Pull Request
#
# Este workflow demonstra o modelo "assistivo" da taxonomia Orqen:
# o agente trabalha de forma autônoma, mas com checkpoints humanos
# em pontos críticos de decisão. Ideal para projetos onde a qualidade
# e a arquitetura precisam de validação antes da implementação.
#
# O agente implementa, mas o humano decide em pontos de inflexão.

agents:
  default: "claude"
  clients:
    qwen:
      command: ["qwen", "--yolo", "--acp"]
    claude:
      command: ["npx", "-y", "@agentclientprotocol/claude-agent-acp"]

execution:
  max_agents: 3
  sleep_interval_seconds: 30

modules:
  - name: task
    dir: "./tasks"
    prefix: "DEV"
    order: ["revisao", "implementacao", "especificacao", "inbox", "pr_aberto"]
    extra_prompt: |
      **CONTEXTO**: Workflow de desenvolvimento assistido. O agente recebe issues/tarefas,
      analisa, especifica a solução, implementa e abre PR — mas com checkpoints humanos
      antes de codificar e antes de abrir o PR.

      **PRINCÍPIO**: O agente faz o trabalho pesado (análise, implementação, testes),
      mas o humano toma as decisões de arquitetura e aprova o resultado final.

      **QUALIDADE**: Todo código deve seguir as convenções do projeto, ter testes e
      passar em linting antes de ser movido para revisão.

      **TEMPLATES**: Para cada artefato gerado, USE O TEMPLATE CORRESPONDENTE no diretório
      `.orqen/task/prompts/` como base estrutural. Preencha todas as seções do template.
      Os templates são:
      - ISSUE.md        → Descrição do problema ou feature request
      - SPEC.md         → Especificação técnica da solução
      - SUMMARY.md      → Resumo da implementação
      - PR.md           → Descrição do Pull Request

    lanes:
      # ── 01_inbox ───────────────────────────────────────────────
      - name: "inbox"
        purpose: "Capturar issues, bugs, feature requests — entrada de trabalho para o agente analisar"
        user_action: "criar issues"
        artifacts: ["ISSUE"]
        max_agents: 1
        agent_behavior:
          - "Leia o arquivo de inbox para entender o problema ou feature request"
          - "Se a descrição for ambígua ou incompleta, escreva perguntas de esclarecimento no arquivo e finalize"
          - "Classifique o tipo: bug, feature, refactor, documentação"
          - "Avalie a complexidade estimada: baixa (1 arquivo), média (2-5 arquivos), alta (6+ arquivos ou múltiplos módulos)"
          - "Identifique dependências: outros módulos, bibliotecas externas, APIs"
          - "Use o tool orqen_create_item (orqen MCP Server) para criar um novo item na lane revisar_issue"
          - "Escreva a análise no arquivo DEV-${SEQUENCE}-ISSUE.md usando o template em '.orqen/task/prompts/ISSUE.md'"
          - "Mova o arquivo de inbox para o diretório do item criado na lane revisar_issue"
          - "Finalize a execução"
        critical_rules:
          - "NUNCA assumir requisitos não explícitos — se houver dúvida, pergunte"
          - "Se o problema for complexo demais para uma única tarefa, sugira decomposição"
        extra_prompt: |
          A qualidade da análise inicial determina a qualidade de toda a cadeia.
          Seja rigoroso na compreensão do problema antes de avançar.

      # ── 02_revisar_issue ───────────────────────────────────────
      - name: "revisar_issue"
        purpose: "Usuário revisa a análise da issue e aprova antes de especificar a solução"
        user_action: "revisar e aprovar issue"

      # ── 03_especificacao ───────────────────────────────────────
      - name: "especificacao"
        purpose: "Analisar a codebase e especificar a solução técnica detalhada — o QUE fazer e COMO, sem implementar ainda"
        artifacts: ["SPEC"]
        max_agents: 1
        ignore_if_dependency: ["inbox"]
        agent_behavior:
          - "Leia DEV-${SEQUENCE}-ISSUE.md para entender o problema aprovado"
          - "Explore a codebase relevante: leia arquivos, interfaces, contratos e padrões existentes"
          - "Identifique os arquivos que serão modificados ou criados"
          - "Defina a abordagem técnica: padrões a seguir, interfaces a respeitar, contratos a manter"
          - "Liste riscos e trade-offs da abordagem proposta"
          - "Defina critérios de aceite claros e verificáveis"
          - "Escreva a especificação no arquivo DEV-${SEQUENCE}-SPEC.md usando o template em '.orqen/task/prompts/SPEC.md'"
          - "Use o tool orqen_move_item (orqen MCP Server) para mover o item da lane especificacao para revisar_especificacao"
          - "Finalize a execução"
        critical_rules:
          - "NÃO implemente nada nesta lane — apenas especifique"
          - "A especificação deve ser detalhada o suficiente para outra pessoa implementar sem dúvidas"
          - "Identifique claramente o que será modificado vs. o que será criado do zero"
        extra_prompt: |
          Esta é a lane de DECISÃO. O humano vai validar a abordagem aqui.
          Seja explícito sobre trade-offs — "podemos fazer X (mais simples) ou Y (mais robusto)".
          Inclua exemplos de código se necessário para ilustrar a abordagem.

      # ── 04_revisar_especificacao ───────────────────────────────
      - name: "revisar_especificacao"
        purpose: "CHECKPOINT HUMANO — Usuário valida a abordagem técnica antes da implementação"
        user_action: "aprovar especificação"

      # ── 05_implementacao ───────────────────────────────────────
      - name: "implementacao"
        purpose: "Implementar a solução conforme especificação aprovada — código, testes e documentação"
        artifacts: ["SUMMARY"]
        max_agents: 1
        ignore_if_exists: ["revisar_especificacao"]
        ignore_if_dependency: ["inbox", "especificacao"]
        agent_behavior:
          - "Leia DEV-${SEQUENCE}-ISSUE.md e DEV-${SEQUENCE}-SPEC.md para entender o problema e a solução aprovada"
          - "Se o arquivo DEV-${SEQUENCE}-FAIL.md existir, leia para entender o problema que foi reportado durante a revisao"
          - "Implemente o código conforme a especificação, seguindo as convenções do projeto"
          - "Se houver críticas de revisão no arquivo DEV-${SEQUENCE}-FAIL.md, faça os ajustes necessários"
          - "Escreva testes unitários e/ou de integração para cobrir os critérios de aceite"
          - "Execute os testes e garanta que todos passam"
          - "Execute linting e correção de estilo se aplicável"
          - "Atualize documentação se a mudança afetar APIs ou comportamento público"
          - "Crie o resumo da implementação no arquivo DEV-${SEQUENCE}-SUMMARY.md usando o template em '.orqen/task/prompts/SUMMARY.md'"
          - "Use o tool orqen_move_item (orqen MCP Server) para mover o item da lane implementacao para revisao"
          - "Finalize a execução"
        critical_rules:
          - "SIGA a especificação aprovada — não desvie sem justificativa documentada"
          - "TODO código novo DEVE ter testes"
          - "NUNCA commitar código que falha nos testes ou linting"
          - "Se encontrar um bloqueio técnico intransponível, documente no SUMMARY e mova para blocked"
        extra_prompt: |
          Implementação assistida: você tem autonomia para codificar, mas a especificação
          foi aprovada pelo humano. Se descobrir algo que invalida a spec durante a implementação,
          documente a divergência e justifique a mudança no SUMMARY.

      # ── 06_revisao ─────────────────────────────────────────────
      - name: "revisao"
        purpose: "Validar a implementação contra a especificação e os critérios de aceite"
        artifacts: ["FAIL"]
        max_agents: 1
        agent_behavior:
          - "Leia DEV-${SEQUENCE}-ISSUE.md, DEV-${SEQUENCE}-SPEC.md e DEV-${SEQUENCE}-SUMMARY.md"
          - "Valide CADA critério de aceite da especificação - liste evidências de conformidade"
          - "Revise a qualidade do código: clareza, organização, segurança, performance"
          - "Verifique se os testes cobrem os cenários definidos"
          - "Verifique se a documentação foi atualizada se necessário"
          - "Se TUDO estiver conforme, use o tool orqen_move_item para mover para pr_aberto"
          - "Se houver problemas, crie o arquivo DEV-${SEQUENCE}-FAIL.md com os rejeições detalhadas"
          - "Se houver problemas, use o tool orqen_move_item (orqen MCP Server) para mover o item da lane revisao para blocked"
          - "Se não houver problemas, use o tool orqen_move_item (orqen MCP Server) para mover o item da lane revisao para pr_aberto"
          - "Finalize a execução"
        critical_rules:
          - "Não aprove se QUALQUER critério de aceite não for atendido"
          - "Documente cada falha com evidência específica (arquivo, linha, comportamento esperado vs. real)"
        extra_prompt: |
          Revisão automatizada de qualidade. Seja rigoroso — o humano confiará nesta revisão
          para decidir se abre o PR.

      # ── 07_blocked ─────────────────────────────────────────────
      - name: "blocked"
        purpose: "Tarefas bloqueadas por impedimentos técnicos que requerem intervenção humana"
        user_action: "resolver bloqueios"
        artifacts: ["FAIL"]

      # ── 08_pr_aberto ───────────────────────────────────────────
      - name: "pr_aberto"
        purpose: "Gerar descrição do Pull Request e preparar para submissão"
        artifacts: ["PR"]
        max_agents: 1
        agent_behavior:
          - "Leia DEV-${SEQUENCE}-ISSUE.md, DEV-${SEQUENCE}-SPEC.md e DEV-${SEQUENCE}-SUMMARY.md"
          - "Gere uma descrição de PR completa com: contexto, mudanças, testes executados, screenshots se aplicável"
          - "Escreva a descrição no arquivo DEV-${SEQUENCE}-PR.md usando o template em '.orqen/task/prompts/PR.md'"
          - "Liste os comandos para criar o PR (git push, gh pr create)"
          - "Use o tool orqen_move_item (orqen MCP Server) para mover o item para concluido"
          - "Finalize a execução"
        critical_rules:
          - "A descrição do PR deve ser autocontida — um revisor humano deve entender sem ler a spec"
          - "Inclua instruções claras de como testar a mudança"
        extra_prompt: |
          O PR é a interface entre o agente e o revisor humano. Faça uma descrição clara,
          concisa e completa.

      # ── 09_concluido ───────────────────────────────────────────
      - name: "concluido"
        purpose: "Tarefas concluídas — PR aberto e pronto para merge"
        user_action: "fazer merge e arquivar"
```

</details>

### Resumo da Configuração

| Campo | Valor |
|-------|-------|
| Prefixo | `DEV` |
| Agente padrão | `qwen` |
| Lanes | 9 (inbox, revisar_issue, especificacao, revisar_especificacao, implementacao, revisao, blocked, pr_aberto, concluido) |
| Artefatos | `ISSUE`, `SPEC`, `SUMMARY`, `PR` |

### Lanes Principais

| Lane | Quem atua | Descrição |
|------|-----------|-----------|
| `inbox` | Agente | Analisa bug/feature e cria issue estruturada |
| `revisar_issue` | Humano | Revisa e aprova a issue |
| `especificacao` | Agente | Cria especificação técnica detalhada |
| `revisar_especificacao` | Humano | Aprova a abordagem técnica |
| `implementacao` | Agente | Implementa conforme spec |
| `revisao` | Agente | Valida critérios de aceitação e DoD |
| `blocked` | Humano | Resolve bloqueios técnicos |
| `pr_aberto` | Agente | Gera descrição do Pull Request |
| `concluido` | Arquivo | Tarefas concluídas, aguardando merge |

## Artefatos

### ISSUE (`.orqen/task/prompts/ISSUE.md`)
Template para bug/feature/refactor. Inclui: tipo, descrição do problema, contexto, complexidade estimada, dependências identificadas, critérios de aceite (alto nível), perguntas de esclarecimento, observações.

### SPEC (`.orqen/task/prompts/SPEC.md`)
Especificação técnica. Inclui: contexto, abordagem técnica, arquivos a modificar, arquivos a criar, design decisions (tabela com opções consideradas e justificativa), riscos e trade-offs, critérios de aceite, estratégia de testes, impacto em documentação, alternativas rejeitadas.

### SUMMARY (`.orqen/task/prompts/SUMMARY.md`)
Resumo da implementação. Inclui: referência (Issue + Spec), o que foi implementado, arquivos modificados (tabela), arquivos criados (tabela), critérios de aceite - status (tabela com evidências), testes, divergências da especificação, bloqueios, próximos passos.

### PR (`.orqen/task/prompts/PR.md`)
Descrição do Pull Request. Inclui: contexto, o que mudou, principais alterações, como testar (com exemplo bash), checklist, screenshots/logs, referências (links para ISSUE, SPEC, SUMMARY), riscos conhecidos.

## Como usar

1. **Baixe** o arquivo [example-pt-dev-assistido.zip](https://github.com/nidorx/orqen/releases/download/__VERSION__/example-pt-dev-assistido.zip)
2. **Ajuste** o `orqen.yaml` conforme sua necessidade
3. **Execute** o Orqen no diretório do projeto
   ```bash
   ./orqen
   ```
4. Crie um arquivo de ideia na lane `01_inbox/`:
   ```
   meu-projeto/.orqen/tasks/01_inbox/minha-feature.md
   ```

## Quando usar

- Projetos que exigem **supervisão humana** em decisões
- Equipes com **processo de review** estabelecido
- Features complexas que precisam de **especificação antes de implementar**
- Quando você quer **controle** sem fazer o trabalho manual

## Próximos passos

- [Dev Autônomo](exemplos/dev-autonomo.md) - Loop fechado sem intervenção
- [Voltar aos exemplos](exemplos.md)
- [Criar Workflow](criar-workflow.md) - Crie seu próprio workflow
