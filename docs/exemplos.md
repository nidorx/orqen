# Exemplos

O Orqen vem com **4 exemplos prontos** em português. Cada exemplo demonstra um tipo de workflow diferente - do desenvolvimento assistido à automação total via Telegram.

## Como usar os exemplos

1. **Baixe** o arquivo do exemplo para seu projeto
2. **Ajuste** o `orqen.yaml` conforme sua necessidade
3. **Execute** o Orqen no diretório do projeto

## Exemplos Disponíveis

### 🔧 Dev Assistido
**Desenvolvimento com checkpoints humanos.** O agente faz o trabalho pesado, mas o humano decide nos pontos críticos (revisão de issue, aprovação de spec, merge).

- **Pipeline:** Issue → Issue Review → Spec → Spec Review → Implementação → Review → PR → Concluído
- **Nível de automação:** Assistido (humano no controle)
- **Ideal para:** Projetos que exigem supervisão humana em decisões de arquitetura
- → [Ver detalhes](exemplos/dev-assistido.md)

### 🤖 Dev Autônomo
**Desenvolvimento em loop fechado.** O agente recebe um objetivo, decompõe, implementa, testa e faz commit - sem intervenção humana.

- **Pipeline:** Objetivo → Decomposição → Implementação → Testes → Commit → Concluído
- **Nível de automação:** Autônomo (sem intervenção)
- **Ideal para:** Projetos menos críticos onde velocidade importa mais que pré-revisão
- → [Ver detalhes](exemplos/dev-autonomo.md)

### 📰 Conteúdo Medium
**Pipeline de artigos longos para Medium.** Pesquisa profunda, rascunho estruturado e revisão rigorosa.

- **Pipeline:** Ideação → Revisão de Ideia → Pesquisa → Rascunho → Revisão → Publicação
- **Nível de automação:** Assistido (revisão humana do artigo)
- **Ideal para:** Artigos técnicos longos (1500+ palavras) com referências verificáveis
- → [Ver detalhes](exemplos/conteudo-medium.md)

### 📱 Portal Autônomo
**Portal de automação via Telegram.** Pedidos chegam pelo bot, o agente processa (gera, build, deploy) e notifica resultados.

- **Pipeline:** Pedido (Telegram) → Gerando → Build → Deploy → Deploy Realizado
- **Nível de automação:** Total (controle remoto via Telegram)
- **Ideal para:** Projetos que precisam de automação end-to-end com notificações
- → [Ver detalhes](exemplos/portal-autonomo.md)

## Comparação Rápida

| Exemplo | Automação | Checkpoints Humanos | Integração Externa |
|---------|-----------|---------------------|-------------------|
| Dev Assistido | Assistido | Issue, Spec, Merge | - |
| Dev Autônomo | Autônomo | Nenhum | - |
| Conteúdo Medium | Assistido | Ideia, Artigo | - |
| Portal Autônomo | Total | Nenhum | Telegram Bot |

## Próximos passos

- [Criar Workflow](criar-workflow.md) - Crie seu próprio workflow customizado
- [Configuração](configuracao.md) - Referência de configuração
- [Conceitos](conceitos.md) - Entenda lanes, módulos e artefatos
