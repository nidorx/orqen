# Ferramentas Dinâmicas (MCP)

Ferramentas dinâmicas permitem definir **ferramentas executáveis personalizadas** diretamente no arquivo `orqen.yaml`. Cada ferramenta definida aparece automaticamente como uma ferramenta MCP disponível para os agentes conectados.

## Visão Geral

Com ferramentas dinâmicas, você pode expor scripts, CLI tools e workflows como ferramentas MCP sem escrever código Go ou configurar servidores MCP externos. O Orqen lê a seção `tools` do `orqen.yaml` e registra automaticamente cada entrada como uma ferramenta MCP com schema tipado.

```
┌─────────────────────────────────────────────────────┐
│  orqen.yaml                                          │
│                                                       │
│  tools:                                               │
│    deploy:                                            │
│      command: ["./deploy.sh", "$env", "$version"]     │
│      args:                                            │
│        env: "Ambiente (staging/prod)"                 │
│        version: "Versão do deploy"                    │
│                                                       │
└──────────────────────┬────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────┐
│  Agente MCP conecta via HTTP                         │
│                                                       │
│  Ferramentas disponíveis:                             │
│  - item                                               │
│  - workitem_move                                      │
│  - fs_list                                            │
│  - deploy  ← sua ferramenta dinâmica                  │
│  - ...                                                │
└─────────────────────────────────────────────────────┘
```

## Estrutura de Configuração

A seção `tools` fica no nível raiz do `orqen.yaml`:

```yaml
tools:
  nome_da_ferramenta:
    command: ["comando", "arg1", "arg2"]
    description: "Descrição da ferramenta"
    args:
      param1: "Descrição do parâmetro 1"
      param2: "Descrição do parâmetro 2"
```

### Campos

| Campo | Obrigatório | Descrição |
|-------|-------------|-----------|
| `command` | Sim | Array de strings com o comando e argumentos padrão |
| `description` | Não | Descrição exibida aos agentes MCP |
| `args` | Não | Mapa de nome do parâmetro → descrição (todos são strings obrigatórias) |
| `timeout` | Não | Timeout em segundos para execução (padrão: **30**) |
| `windows` | Não | Override do comando para Windows |
| `darwin` | Não | Override do comando para macOS |
| `linux` | Não | Override do comando para Linux |

> **Importante:** O nome da ferramenta (chave no YAML) **não pode** conflitar com ferramentas embutidas do Orqen. Conflitos são ignorados com um aviso no log.

## Substituição de Wildcards

Os argumentos no campo `command` suportam substituição de wildcards usando a sintaxe `$nome_parametro`. Quando o agente invoca a ferramenta, cada ocorrência exata de `$nome_parametro` é substituída pelo valor fornecido.

### Regras de Substituição

- Apenas **tokens exatos** são substituídos: `$param` funciona, mas `prefixo_$param` **não**
- A correspondência é **case-sensitive**: `$Param` ≠ `$param`
- Todos os parâmetros definidos em `args` são **obrigatórios**

### Exemplo

```yaml
tools:
  saudacao:
    command: ["echo", "Olá", "$nome", "da equipe", "$equipe"]
    args:
      nome: "Nome da pessoa"
      equipe: "Nome da equipe"
```

Quando o agente chama `saudacao(nome="Ana", equipe="Backend")`, o comando executado é:

```bash
echo Olá Ana da equipe Backend
```

## Estilos de Parâmetros

### Parâmetros Nomeados (Flags)

Use quando o comando aceita flags estilo `--nome valor`:

```yaml
tools:
  buscar_arquivo:
    command: ["find", ".", "-name", "$padrao", "-type", "$tipo"]
    description: "Busca arquivos por nome e tipo"
    args:
      padrao: "Padrão de busca (ex: '*.go')"
      tipo: "Tipo de arquivo (f para arquivo, d para diretório)"
```

### Parâmetros Posicionais

Use quando o comando aceita argumentos em ordem fixa:

```yaml
tools:
  executar_script:
    command: ["bash", "$script", "$ambiente"]
    description: "Executa um script bash em um ambiente"
    args:
      script: "Caminho do script"
      ambiente: "Ambiente alvo (dev/staging/prod)"
```

## Overrides por Sistema Operacional

Para comandos que diferem entre sistemas operacionais, use os campos `windows`, `darwin` e `linux`:

```yaml
tools:
  limpar_cache:
    command: ["rm", "-rf", "./cache/*"]
    windows: ["cmd", "/c", "rmdir", "/s", "/q", "cache"]
    darwin: ["rm", "-rf", "./cache/*"]
    linux: ["rm", "-rf", "./cache/*"]
    description: "Limpa o diretório de cache"
```

O Orqen seleciona automaticamente o comando adequado ao SO em que está executando. Se não houver override para o SO atual, usa o `command` padrão.

> **Dica:** No Windows, use `cmd /c` para comandos nativos ou `powershell` para scripts PowerShell.

## Exemplos Práticos

### Exemplo 1: Execução de Script com Múltiplos Parâmetros

```yaml
tools:
  deploy_app:
    command: ["./scripts/deploy.sh", "--env", "$ambiente", "--version", "$versao", "--dry-run", "$dry_run"]
    windows: ["scripts\\deploy.bat", "/env", "$ambiente", "/version", "$versao", "/dry-run", "$dry_run"]
    timeout: 120
    description: "Executa o pipeline de deploy da aplicação"
    args:
      ambiente: "Ambiente alvo: staging, production"
      versao: "Tag ou branch do Git para deploy"
      dry_run: "Simular deploy sem aplicar (true/false)"
```

Este exemplo demonstra:
- Uso de flags nomeadas (`--env`, `--version`)
- Override para Windows com sintaxe diferente
- Timeout customizado (120s)
- Múltiplos parâmetros obrigatórios

### Exemplo 2: Ferramenta de Análise de Código

```yaml
tools:
  analisar_codigo:
    command: ["golangci-lint", "run", "--timeout", "$timeout", "./..."]
    description: "Executa o linter Go no projeto"
    timeout: 180
    args:
      timeout: "Timeout da análise (ex: 5m, 10m)"
```

### Exemplo 3: Geração de Relatório

```yaml
tools:
  gerar_relatorio:
    command: ["python3", "scripts/relatorio.py", "--modulo", "$modulo", "--formato", "$formato", "--output", "$output"]
    description: "Gera relatório de cobertura de testes"
    args:
      modulo: "Nome do módulo para análise"
      formato: "Formato de saída: html, json, markdown"
      output: "Caminho do arquivo de saída"
```

## Considerações de Segurança

### ⚠️ Avisos Importantes

1. **Comandos executam com permissões do diretório do projeto** — Não há sandboxing embutido. Scripts executados por ferramentas dinâmicas têm acesso ao sistema de arquivos do projeto e herdam as permissões do processo Orqen.

2. **Sem sandboxing embutido** — O Orqen não isola a execução de ferramentas dinâmicas. Ferramentas mal configuradas podem modificar ou excluir arquivos do projeto.

3. **Valide entradas de ferramentas voltadas ao público** — Se suas ferramentas são invocadas por agentes que processam input externo, valide os parâmetros no script para evitar injeção de comandos.

### 💡 Dicas de Segurança

- **Restrinja caminhos** — Use caminhos relativos e valide que parâmetros de caminho não escapem do diretório esperado (ex: rejeite `../` em paths)
- **Use timeouts adequados** — Defina `timeout` para evitar que comandos travados consumam recursos indefinidamente
- **Prefira scripts versionados** — Aponte para scripts no repositório em vez de comandos arbitrários
- **Revise permissões** — Garanta que os scripts tenham apenas as permissões necessárias (ex: `chmod +x` apenas nos scripts necessários)
- **Não exponha credenciais** — Evite passar senhas ou tokens como parâmetros visíveis no log. Use variáveis de ambiente ou arquivos de configuração protegidos

## Quando Usar Ferramentas Dinâmicas vs MCP Servers

| Critério | Ferramentas Dinâmicas | MCP Servers |
|----------|----------------------|-------------|
| **Complexidade** | Scripts simples, CLI tools | Aplicações com estado, APIs |
| **Configuração** | YAML inline no `orqen.yaml` | Servidor externo separado |
| **Manutenção** | Embutida no projeto do Orqen | Infraestrutura independente |
| **Retorno** | stdout/stderr do comando | Respostas estruturadas JSON |
| **Casos de uso** | Scripts de deploy, linters, geradores de relatório | Bancos de dados, APIs externas, sistemas com autenticação |

**Use ferramentas dinâmicas quando:**
- O comando é simples e executa rapidamente (segundos a poucos minutos)
- Você quer expor um script existente sem infraestrutura adicional
- O retorno em texto (stdout/stderr) é suficiente

**Use MCP servers quando:**
- A ferramenta precisa manter estado entre chamadas
- A ferramenta requer autenticação ou conexão com serviços externos
- O retorno precisa ser estruturado (JSON, tipos complexos)
- A ferramenta é complexa o suficiente para merecer código dedicado

## Referência do Arquivo

Para a referência completa da seção `tools` e outros campos do `orqen.yaml`, consulte:

→ [Referência Completa de Configuração](https://github.com/nidorx/orqen/blob/main/docs/CONFIG.md)

## Voltar

- [Configuração](configuracao.md)
- [Início](/)
