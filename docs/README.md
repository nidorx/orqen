## O que é

Orqen é um motor/**en**gine de **orq**uestração de workflows open-source escrito em **Go**. Inspirado no modelo Kanban, ele transforma processos repetitivos em pipelines claros com memória persistente e execução determinística.

Funciona com [**qualquer agente que use o Agent Client Protocol (ACP)**](https://agentclientprotocol.com/get-started/agents), dando liberdade para escolher provedores de IA (Qwen, Claude, GitHub Copilot, etc.).

## Para que serve

| Uso | Exemplo |
|-----|---------|
| Desenvolvimento de Software | Backlog → Implementação → Revisão → Deploy |
| Criação de Conteúdo | Ideação → Pesquisa → Rascunho → Revisão → Publicação |
| Pipelines de Marketing | Planejamento → Aprovação → Execução → Métricas |
| Qualquer Processo Repetitivo | Se você faz repetidamente, Orqen estrutura |

## Como funciona (resumo)

1. **Configura** - Um único arquivo `.orqen/orqen.yaml` define o workflow
2. **Escaneia** - O motor verifica lanes em ordem de prioridade
3. **Invoca** - Um agente ACP recebe contexto + instruções + MCP
4. **Executa** - O agente age no item, cria artefatos, move para outras lanes
5. **Repete** - O ciclo continua em intervalo configurável

## Próximo passos

- [Por que Orqen](por-que-orqen.md) - Entenda o problema que o Orqen resolve
- [Instalação](instalacao.md) - Instale e execute em minutos
- [Conceitos](conceitos.md) - Lanes, módulos, artefatos, ACP
- [Exemplos](exemplos.md) - Pipelines prontos para usar
