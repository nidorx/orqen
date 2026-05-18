# Integração com Telegram

O Orqen pode ser controlado remotamente via um bot do Telegram. Pedidos chegam pelo bot, o agente processa e notifica os resultados de volta.

Além de executar comando, a integração com o Telegram permite que você possa conversar com o agente que tem acesso ao seu projeto, permitindo solicitar análises remota.

> Em desenvolvimento, espere BUGs, contribua!

```
┌──────────┐     ┌─────────┐     ┌──────────────┐     ┌──────────┐     ┌──────────┐
│ Usuário  │────>│  Bot    │────>│ Orqen Engine │────>│ Agente   │────>│ Usuário  │
│ Telegram │     │Telegram │     │              │     │ ACP      │     │ Telegram │
└──────────┘     └─────────┘     └──────────────┘     └──────────┘     └──────────┘
     │                                                      │
     │◄─────────────── Notificação de status ───────────────│
     │               (via Orqen Engine → Bot)               │
```

## Criando o Bot

### Passo 1: Abra o BotFather

No Telegram, busque por `@BotFather` e inicie uma conversa.

### Passo 2: Crie um novo bot

Envie o comando:
```
/newbot
```

### Passo 3: Configure o bot

1. **Nome do bot** - Nome exibido (ex: `Orqen Bot`)
2. **Username do bot** - Deve terminar em `bot` (ex: `meu_orqen_bot`)

### Passo 4: Copie o token

Após criar, o BotFather retorna um token:
```
Use this token to access the HTTP API:
8279208708:AAE***********7n_0
```

> **Segurança:** Nunca compartilhe ou commit o token em repositórios públicos.

## Configurando no Orqen

Adicione a seção `chat` ao seu `orqen.yaml`:

```yaml
chat:
  telegram:
    token: "SEU_TOKEN_AQUI"
    secret: "123"    # Secret para autenticação (opcional mas recomendado)
```

| Campo | Obrigatório | Descrição |
|-------|-------------|-----------|
| `token` | Sim | Token do bot fornecido pelo BotFather |
| `secret` | Não | Secret para validação de acesso (use um valor aleatório em produção) |

### Exemplo completo

```yaml
agents:
  default: "qwen"
  clients:
    qwen:
      command: ["qwen", "--yolo", "--acp"]

execution:
  max_agents: 10
  sleep_interval_seconds: 60

chat:
  telegram:
    token: "8279208708:AAE***********7n_0"

modules:
  - name: task
    dir: "tasks"
    lanes:
      - name: "pedido"
        purpose: "Pedido recebido via Telegram"
        # ...
```

## Segurança

- **Nunca commite tokens** em repositórios públicos
- Use variáveis de ambiente para o token em produção:
  ```yaml
  chat:
    telegram:
      token: "${TELEGRAM_BOT_TOKEN}"
  ```
- Configure o `secret` para autenticação do acesso
- Restrinja o bot a chats específicos se necessário

## Próximos passos

- [Exemplos](exemplos.md) - Pipelines prontos para usar
- [Portal Autônomo](exemplos/portal-autonomo.md) - Exemplo completo com Telegram
- [Configuração](configuracao.md) - Referência de configuração
