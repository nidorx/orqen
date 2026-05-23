# Exemplo: Portal Autônomo

> **IMPORTANTE** Ainda em fase de implementaçao

**Nível máximo de automação.** Pedidos chegam via Telegram, o agente processa tudo (geração, build, deploy) e notifica os resultados de volta ao usuário.

## Pipeline

```
┌────────────────┐    ┌──────────┐    ┌────────┐    ┌────────┐    ┌─────────────────┐
│ PEDIDO         │───>│ GERANDO  │───>│ BUILD  │───>│ DEPLOY │───>│ DEPLOY REALIZADO│
│ (Telegram)     │    │ (agent)  │    │(agent) │    │(agent) │    │   (arquivar)    │
└────────────────┘    └──────────┘    └────────┘    └────────┘    └─────────────────┘
       │                                                              │
       │◄──────────── Notificações Telegram em cada transição ───────│
```

**Sem checkpoints humanos.** Controle total via comandos Telegram.

## Comandos Telegram

| Comando | Descrição | Exemplo |
|---------|-----------|---------|
| `/novo` | Cria novo pedido | `/novo adicionar autenticação OAuth` |
| `/status` | Consulta status | `/status` ou `/status PORTAL-0001` |
| `/cancel` | Cancela pedido | `/cancel PORTAL-0001` |
| `/ajuda` | Mostra ajuda | `/ajuda` |

## Configuração

<details>
<summary>Clique para ver o orqen.yaml completo</summary>

```yaml
# Orqen Workflow - Portal Autônomo com Interação Remota via Telegram
# Pipeline: Pedido (Telegram) → Geração → Build → Deploy
#
# Este workflow demonstra o "Tipo 9" da taxonomia Orqen - o nível mais alto
# de automação: um pedido chega via Telegram, o agente processa tudo
# (geração, build, deploy) e notifica o resultado de volta no Telegram.
#
# O usuário interage remotamente enviando mensagens para o bot do Telegram.
# O Orqen escuta, cria itens na lane de entrada, processa e responde.
#
# CONFIGURAÇÃO DO TELEGRAM:
# - Crie um bot via @BotFather no Telegram
# - Obtenha o TOKEN do bot
# - Configure abaixo em agents.telegram.token
# - O Orqen usará o Telegram como interface de comando remoto
#
# COMANDOS SUPORTADOS PELO BOT:
#   /novo <descrição>  - Cria um novo pedido de conteúdo/feature
#   /status            - Lista itens em processamento
#   /cancel <ID>       - Cancela um item em processamento
#   /ajuda             - Lista comandos disponíveis

agents:
  default: "claude"
  clients:
    qwen:
      command: ["qwen", "--yolo", "--acp"]
    claude:
      command: ["npx", "-y", "@agentclientprotocol/claude-agent-acp"]

execution:
  max_agents: 3
  sleep_interval_seconds: 15

chat:
  telegram:
    # Token do bot criado via @BotFather
    # Substitua pelo token real do seu bot
    token: "SEU_TOKEN_AQUI"
    # Chat ID do usuário/administrador (opcional - para notificações diretas)
    # admin_chat_id: "123456789"

modules:
  - name: task
    dir: "./tasks"
    prefix: "PORTAL"
    order: ["deploy", "build", "gerando", "pedido", "deploy_realizado"]
    extra_prompt: |
      **CONTEXTO**: Portal autônomo com interação remota via Telegram.
      Pedidos chegam como mensagens do Telegram, são processados pelo Orqen
      e o resultado é notificado de volta no chat.

      **INTEGRAÇÃO COM TELEGRAM**:
      - Quando um item é criado na lane 'pedido', o Orqen recebeu via Telegram
      - Ao finalizar cada lane, notifique o usuário no Telegram com status
      - Use comandos de notificação para informar progresso

      **NOTIFICAÇÕES TELEGRAM**:
      - **Início**: "🔄 Pedido PORTAL-XXXX iniciado: <descrição>"
      - **Em progresso**: "⏳ PORTAL-XXXX: gerando conteúdo..."
      - **Build**: "🔨 PORTAL-XXXX: build em andamento..."
      - **Deploy**: "🚀 PORTAL-XXXX: deploy realizado com sucesso!"
      - **Falha**: "❌ PORTAL-XXXX falhou: <motivo>. Intervenção necessária."

      **TEMPLATES**: Para cada artefato gerado, USE O TEMPLATE CORRESPONDENTE no diretório
      `.orqen/task/prompts/` como base estrutural. Preencha todas as seções do template.
      Os templates são:
      - PEDIDO.md       → Pedido recebido via Telegram
      - GERADO.md       → Conteúdo/código gerado
      - BUILD.md        → Resultado do build
      - DEPLOY.md       → Resultado do deploy

    lanes:
      # ── 00_pedido ──────────────────────────────────────────────
      - name: "pedido"
        purpose: "Pedidos recebidos via Telegram - entrada do portal autônomo"
        artifacts: ["PEDIDO"]
        max_agents: 1
        agent_behavior:
          - "Leia o arquivo de pedido para entender a solicitação recebida via Telegram"
          - "Valide se o pedido é claro e executável autonomamente"
          - "Se for ambíguo, responda no Telegram pedindo esclarecimento e finalize"
          - "Classifique o tipo: conteúdo, feature, bugfix, deploy"
          - "Identifique o projeto/repositório alvo (se aplicável)"
          - "Escreva o pedido estruturado no arquivo PORTAL-${SEQUENCE}-PEDIDO.md usando o template em '.orqen/task/prompts/PEDIDO.md'"
          - "Notifique no Telegram: '📋 Pedido PORTAL-${SEQUENCE} recebido: <descrição curta>'"
          - "Use the tool orqen_move_item (orqen MCP Server) para mover o item da lane pedido para gerando"
          - "Finalize a execução"
        critical_rules:
          - "Se o pedido envolver operações destrutivas (delete, drop, force push), rejeite e notifique no Telegram"
          - "Se o pedido for fora do escopo do portal, explique o limite e finalize"
        extra_prompt: |
          Esta é a porta de entrada. O pedido veio de uma mensagem do Telegram - pode ser
          informal, incompleto ou ambíguo. Sua job é estruturar e validar antes de executar.

      # ── 01_gerando ─────────────────────────────────────────────
      - name: "gerando"
        purpose: "Gerar o conteúdo, código ou artefato solicitado - o trabalho principal do agente"
        artifacts: ["GERADO"]
        max_agents: 2
        agent_behavior:
          - "Leia PORTAL-${SEQUENCE}-PEDIDO.md para entender o que foi solicitado"
          - "Execute a geração conforme o pedido: escreva código, crie conteúdo, modifique arquivos"
          - "Siga as convenções e padrões do projeto alvo"
          - "Se o pedido envolver múltiplos arquivos, organize-os corretamente"
          - "Documente o que foi gerado e qualquer decisão tomada durante o processo"
          - "Escreva o resumo no arquivo PORTAL-${SEQUENCE}-GERADO.md usando o template em '.orqen/task/prompts/GERADO.md'"
          - "Notifique no Telegram: '⏳ PORTAL-${SEQUENCE}: geração concluída, iniciando build...'"
          - "Use the tool orqen_move_item (orqen MCP Server) para mover o item da lane gerando para build"
          - "Finalize a execução"
        critical_rules:
          - "NUNCA gerar código com secrets, tokens ou credenciais hardcoded"
          - "Se a geração falhar com bloqueio técnico, notifique no Telegram e mova para falha"
        extra_prompt: |
          Geração autônoma: você tem liberdade para tomar decisões técnicas, mas documente
          tudo no artefato GERADO. O usuário pode perguntar "por que fez assim?" depois.

      # ── 02_build ───────────────────────────────────────────────
      - name: "build"
        purpose: "Executar o build/compilação/validação do que foi gerado"
        artifacts: ["BUILD"]
        max_agents: 1
        agent_behavior:
          - "Leia PORTAL-${SEQUENCE}-PEDIDO.md e PORTAL-${SEQUENCE}-GERADO.md"
          - "Execute o build do projeto: go build, npm run build, docker build, etc."
          - "Execute os testes existentes para garantir que nada foi quebrado"
          - "Execute linting e verificação de estilo"
          - "Se TUDO passar, documente no arquivo PORTAL-${SEQUENCE}-BUILD.md usando o template em '.orqen/task/prompts/BUILD.md'"
          - "Se ALGO falhar, documente os erros e mova para falha"
          - "Notifique no Telegram o resultado do build"
          - "Use the tool orqen_move_item (orqen MCP Server) para mover para deploy (se sucesso) ou falha (se erro)"
          - "Finalize a execução"
        critical_rules:
          - "NUNCA aprovar build com falhas"
          - "NUNCA aprovar se testes existentes falharem"
          - "Inclua output completo do build no artefato - mesmo se passar"
        extra_prompt: |
          Gate de qualidade. Mesmo em modo autônomo, nada vai para deploy sem build passando.

      # ── 03_deploy ──────────────────────────────────────────────
      - name: "deploy"
        purpose: "Realizar o deploy do que foi gerado e validado"
        artifacts: ["DEPLOY"]
        max_agents: 1
        agent_behavior:
          - "Leia PORTAL-${SEQUENCE}-PEDIDO.md, PORTAL-${SEQUENCE}-GERADO.md e PORTAL-${SEQUENCE}-BUILD.md"
          - "Execute o deploy conforme configurado: push para produção, deploy em servidor, publicação"
          - "Verifique que o deploy foi bem-sucedido (health check, smoke test)"
          - "Documente o resultado no arquivo PORTAL-${SEQUENCE}-DEPLOY.md usando o template em '.orqen/task/prompts/DEPLOY.md'"
          - "Notifique no Telegram: '🚀 PORTAL-${SEQUENCE}: deploy realizado com sucesso! URL: <url>'"
          - "Use the tool orqen_move_item (orqen MCP Server) para mover para deploy_realizado"
          - "Finalize a execução"
        critical_rules:
          - "NUNCA fazer deploy sem build passing"
          - "Sempre fazer smoke test pós-deploy"
          - "Se o deploy falhar, notifique imediatamente no Telegram com o erro"
        extra_prompt: |
          Último passo do loop autônomo. O deploy pode ser:
          - git push para branch principal
          - docker push + deploy em Kubernetes
          - upload para S3/CDN
          - publicação em plataforma (Vercel, Netlify, etc.)

          Adapte o comando de deploy ao projeto específico.

      # ── 04_falha ───────────────────────────────────────────────
      - name: "falha"
        purpose: "Pedidos que falharam em alguma etapa - requer intervenção humana via Telegram"
        user_action: "analisar e decidir via Telegram"
        artifacts: ["FAIL"]
        agent_behavior:
          - "Leia os artefatos existentes até o ponto de falha"
          - "Identifique a causa raiz da falha"
          - "Documente detalhadamente no arquivo PORTAL-${SEQUENCE}-FAIL.md"
          - "Notifique no Telegram: '❌ PORTAL-${SEQUENCE} falhou na etapa <etapa>: <motivo>. Responda com /reintentar ou /cancelar'"
          - "Aguarde comando do usuário no Telegram"
          - "Finalize a execução"
        critical_rules:
          - "Sempre notifique no Telegram com detalhes suficientes para o usuário decidir"
          - "Inclua logs e mensagens de erro no artefato de falha"

      # ── 05_deploy_realizado ────────────────────────────────────
      - name: "deploy_realizado"
        purpose: "Pedidos concluídos com deploy realizado - histórico do portal"
        user_action: "revisar e arquivar"
```

</details>

### Resumo da Configuração

| Campo | Valor |
|-------|-------|
| Prefixo | `PORTAL` |
| Agente padrão | `qwen` |
| Lanes | 6 (pedido, gerando, build, deploy, falha, deploy_realizado) |
| Artefatos | `PEDIDO`, `GERADO`, `BUILD`, `DEPLOY` |
| Integração | Telegram Bot |

### Lanes Principais

| Lane | Quem atua | Descrição |
|------|-----------|-----------|
| `pedido` | Agente | Pedido recebido via `/novo`, estrutura e valida |
| `gerando` | Agente | Gera código/conteúdo conforme pedido |
| `build` | Agente | Executa build, testes e lint |
| `deploy` | Agente | Faz deploy e smoke test pós-deploy |
| `falha` | Humano | Analisa falha via Telegram e decide next steps |
| `deploy_realizado` | Arquivo | Deploy concluído com sucesso |

## Artefatos

### PEDIDO (`.orqen/task/prompts/PEDIDO.md`)
Pedido recebido via Telegram. Inclui: origem (chat ID, mensagem original, timestamp), tipo (conteúdo/feature/bugfix/deploy), descrição estruturada, projeto/repo alvo, escopo, critérios de sucesso, checklist de validação.

### GERADO (`.orqen/task/prompts/GERADO.md`)
Resumo do que foi gerado. Inclui: referência ao pedido, o que foi gerado, arquivos criados/modificados, decisões técnicas tomadas, próximos passos, observações.

### BUILD (`.orqen/task/prompts/BUILD.md`)
Resultado do build. Inclui: comando de build executado, output completo, status (PASS/FAIL), testes executados (passed/failed), resultado do linting, duração, observações.

### DEPLOY (`.orqen/task/prompts/DEPLOY.md`)
Resultado do deploy. Inclui: referências (Pedido + Gerado + Build), tipo de deploy, comando executado, output completo, status (SUCCESS/FAIL), smoke test pós-deploy (URL, comando de verificação, resultado), timestamp, commit/tag, ambiente, notificação Telegram enviada, observações.

## Notificações Telegram

O Orqen envia notificações automáticas em cada etapa:

### Pedido Recebido
```
📋 Novo Pedido
ID: PORTAL-0001
Descrição: adicionar autenticação OAuth
Status: Gerando...
```

### Build Concluído
```
🔨 Build PORTAL-0001
Status: ✅ PASS
Testes: 42 passed, 0 failed
Duração: 2m 34s
```

### Deploy Concluído
```
✅ Deploy PORTAL-0001 Realizado
Ambiente: production
URL: https://app.exemplo.com
Commit: abc1234
Smoke test: OK
```

### Falha
```
❌ PORTAL-0001 Falhou
Etapa: build
Erro: tests failed (3 failures)
Ação: revise os testes e reenvie com /novo
```

## Como usar

1. **Baixe** o arquivo [example-pt-portal-autonomo.zip](https://github.com/nidorx/orqen/releases/download/__VERSION__/example-pt-portal-autonomo.zip)
2. **Ajuste** o `orqen.yaml` conforme sua necessidade
3. Configure o token do Telegram no `orqen.yaml` (veja [Telegram](telegram.md))
4. **Execute** o Orqen no diretório do projeto
   ```bash
   ./orqen
   ```
5. Inicie o bot no Telegram `/new` e solicite que o Orqen faça algo como 'Altere o título da página'

## Quando usar

- Projetos que precisam de **automação end-to-end**
- **CI/CD** controlado por chat
- Equipes remotas que querem **acionar deploys por mensagem**
- Quando você confia no agente para **decisões técnicas completas**

## Requisitos Prévios

- Bot do Telegram configurado (veja [Telegram](telegram.md))
- Projeto com script de build configurado
- Pipeline de deploy automatizado (script ou CLI)
- Agente ACP configurado

## Próximos passos

- [Voltar aos exemplos](exemplos.md)
- [Telegram](telegram.md) - Configuração do bot
- [Criar Workflow](criar-workflow.md) - Crie seu próprio workflow
