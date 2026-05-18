# Agentes

Orqen funciona com qualquer agente que suporte o **Agent Client Protocol (ACP)**. Configure o comando de invocação no arquivo `orqen.yaml` e o Orqen gerencia o restante.

## Configuração Básica

```yaml
agents:
  default: "qwen"              # Agente padrão
  clients:
    qwen:
      command: ["qwen", "--yolo", "--acp"]
```

| Campo | Descrição |
|-------|-----------|
| `default` | Nome do agente usado por todas as lanes (pode ser sobrescrito por lane) |
| `clients.<nome>.command` | Array de strings: executável + argumentos |

> O comando é executado como subprocesso. Inclua flags que habilitam **modo autônomo** e **suporte ACP**.

## Qwen (Qwen Code)

### Instalação
```bash
npm install -g @anthropic-ai/qwen-code  # ou conforme distribuição
```

### Configuração
```yaml
agents:
  default: "qwen"
  clients:
    qwen:
      command: ["qwen", "--yolo", "--acp"]
```

| Flag | Descrição |
|------|-----------|
| `--yolo` | Modo autônomo - executa sem pedir permissão |
| `--acp` | Habilita o Agent Client Protocol |

### Notas
- Qwen Code é o agente de referência do Orqen
- A combinação `--yolo --acp` permite execução autônoma com ferramentas MCP

---

## Claude (Claude Code)

### Instalação
```bash
npm install -g @anthropic-ai/claude-code  # ou conforme distribuição
```

### Configuração
```yaml
agents:
  default: "claude"
  clients:
    claude:
      command: ["claude", "--dangerously-skip-permissions", "--acp"]
```

| Flag | Descrição |
|------|-----------|
| `--dangerously-skip-permissions` | Modo autônomo - ignora prompts de permissão |
| `--acp` | Habilita o Agent Client Protocol (se suportado) |

### Notas
- Verifique se a versão do Claude Code suporta ACP
- A flag `--dangerously-skip-permissions` permite execução sem intervenção humana

---

## GitHub Copilot (copilot-cli)

### Instalação
```bash
# Via GitHub CLI extensions ou conforme distribuição
gh extension install github/gh-copilot
```

### Configuração
```yaml
agents:
  default: "copilot"
  clients:
    copilot:
      command: ["copilot", "--acp", "--autonomous"]
```

| Flag | Descrição |
|------|-----------|
| `--acp` | Habilita o Agent Client Protocol |
| `--autonomous` | Modo autônomo (verificar disponibilidade na versão instalada) |

### Notas
- Verifique a versão do copilot-cli para flags de modo autônomo
- O suporte a ACP pode variar por versão

---

## Múltiplos Agentes

Você pode configurar vários agentes e usar o padrão ou sobrescrever por lane:

```yaml
agents:
  default: "qwen"
  clients:
    qwen:
      command: ["qwen", "--yolo", "--acp"]
    claude:
      command: ["claude", "--dangerously-skip-permissions"]
```

### Sobrescrevendo por Lane

```yaml
modules:
  - name: task
    lanes:
      - name: "doing"
        agent: "claude"           # Usa Claude nesta lane específica
        purpose: "Implementação com Claude"

      - name: "review"
        agent: "qwen"             # Usa Qwen para review
        purpose: "Revisão com Qwen"
```

## Requisitos do Agente

Para funcionar com Orqen, o agente deve:

1. **Suportar MCP tools** - O agente precisa receber e usar ferramentas MCP (`orqen_status`, `orqen_item_create`, etc.)
2. **Rodar em modo autônomo** - Não deve pedir permissão para cada ação de filesystem
3. **Aceitar stdin/stdout** - O Orqen invoca o agente como subprocesso

## Verificando a Conexão

Execute o Orqen e observe os logs. Se o agente for invocado corretamente, você verá:

```
[agent] Invoking qwen for task.doing:TASK-0001-implement-auth
[agent] qwen exited with code 0 - COMPLETE
```

Se o agente falhar:
```
[agent] qwen exited with code 1 - FAIL
Error: agent command not found
```

## Próximos passos

- [Telegram](telegram.md) - Integre com bot do Telegram
- [Exemplos](exemplos.md) - Pipelines prontos para usar
- [Configuração](configuracao.md) - Referência de configuração
