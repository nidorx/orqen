# Recuperação de Sessão

Quando uma sessão do Orqen falha durante a execução, o sistema é capaz de recuperar automaticamente a sessão anterior ao retentar a tarefa. Isso evita que o agente perca o contexto acumulado e precise recomeçar do zero.

## Como funciona

O mecanismo de recuperação de sessão opera de forma totalmente automática:

1. **Antes da execução**: O Orqen verifica se o work item possui um atributo `session_id` salvo de uma execução anterior
2. **Se `session_id` existe**: O engine tenta carregar a sessão existente usando o protocolo ACP (`LoadSession`)
3. **Se não existe ou falha**: Uma nova sessão é criada automaticamente
4. **Durante a execução**: O ID da sessão é mantido em memória e associado ao work item
5. **Ao concluir com sucesso**: O atributo `session_id` é removido do work item
6. **Em caso de falha**: O `session_id` é preservado no work item para a próxima tentativa

### Fluxo visual

```
Execução 1 (falha)          Execução 2 (retry)
┌─────────────────┐         ┌─────────────────┐
│ Nova sessão     │         │ session_id      │
│ session_id=abc  │──falha──▶ existente? SIM  │
│ salva em WI     │         │ LoadSession(abc)│
└─────────────────┘         │ continua execução│
                            └─────────────────┘
```

## Integração com ACP

A recuperação de sessão utiliza o protocolo **Agent Client Protocol (ACP)**, especificamente o método `LoadSession`. Para que funcione:

- O agente deve anunciar a capability `loadSession: true` durante o handshake de inicialização
- O Orqen gerencia automaticamente o ciclo de vida da sessão (criação, recuperação e limpeza)
- Não há configuração necessária por parte do usuário

> **Nota**: A documentação técnica do protocolo ACP está disponível no [ACP Go SDK](https://pkg.go.dev/github.com/coder/acp-go-sdk).

## Cenários de Recuperação

| Cenário | Comportamento |
|---------|--------------|
| Agente falha durante execução | `session_id` preservado → retry carrega sessão anterior |
| Sessão expirada no agente | Fallback automático para nova sessão |
| Processo do agente encerrado | Nova sessão criada (sessão anterior perdida) |
| Orqen reiniciado | `session_id` ainda no work item → tenta LoadSession |
| Agente não suporta `LoadSession` | Sempre cria nova sessão (sem impacto negativo) |

## Limitações

- **Capability obrigatória**: Apenas agentes que anunciam `loadSession: true` podem recuperar sessões
- **Tempo de vida da sessão**: A sessão existe enquanto o subprocesso do agente estiver ativo
- **Contexto acumulado**: Se o agente for reiniciado (não apenas a sessão), o contexto de conversa anterior será perdido
- **Fallback automático**: Se `LoadSession` falhar por qualquer motivo, o Orqen cria uma nova sessão automaticamente — nenhuma ação manual é necessária

## Troubleshooting

### A sessão não está sendo recuperada

1. **Verifique se o agente suporta `LoadSession`**: Consulte a documentação do seu agente para confirmar que ele anuncia essa capability
2. **Observe os logs do Orqen**: Logs indicam se `LoadSession` foi tentado e se houve fallback para nova sessão
3. **Verifique o atributo `session_id`**: O Orqen gerencia este atributo automaticamente — se ele não existe no work item, significa que nenhuma execução anterior falhou ou que a execução anterior concluiu com sucesso

### Quando uma nova sessão é criada automaticamente

Uma nova sessão será criada (em vez de recuperar a anterior) quando:
- O work item não possui atributo `session_id`
- O agente não suporta `LoadSession`
- A sessão anterior expirou ou foi encerrada pelo agente
- O processo do agente foi reiniciado entre execuções

## Veja também

- [Agentes](agentes.md) - Configuração de agentes compatíveis com Orqen
- [Configuração](configuracao.md) - Referência completa de configuração
