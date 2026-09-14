# Clipboard e arquivos sem agente — primeiro ajuste

## Limite confirmado: arquivos não são clipboard nativo

Não é possível entregar Ctrl+C no Explorador local e Ctrl+V em uma pasta remota,
e vice-versa, como no mstsc, com a integração web/Guacamole atual. O navegador
não oferece acesso equivalente ao clipboard de arquivos do sistema; receber um
File em certos eventos paste não implementa o canal RDP de clipboard de arquivos.
HTTPS/permissões de clipboard não acrescentam essa funcionalidade.

Alternativa operacional sem agente:

- Local → remoto: arraste o arquivo para a tela ConnectMe (ou use Enviar arquivo).
  Dentro do Windows, abra `\\tsclient\ConnectMe`, copie o arquivo e cole na pasta final.
- Remoto → local: copie o arquivo no Windows para a raiz de `\\tsclient\ConnectMe`.
  No painel, clique Atualizar arquivos e Baixar. Depois mova o download localmente.

Esses Ctrl+C/V dentro do Windows remoto são operações entre pastas remotas,
não sincronização do clipboard de arquivos entre computadores. Uma solução
equivalente ao mstsc exigiria outro cliente/integração nativa e mudanças de
protocolo; não foi implementada nem introduzido agente nesta alteração.

## Correção de prioridade do teclado e botão direito SSH

O handler de Ctrl+V agora captura no `window`, antes do teclado Guacamole
registrado no `document`. A versão anterior era interceptada pelo Guacamole e
não recebia o evento nativo de colagem. No SSH, o atalho enviado ao terminal
foi corrigido para Ctrl+Shift+V (keysym V maiúsculo), não Shift+Insert.

O terminal SSH oferece uma superfície editável transparente para o menu nativo
do navegador: botão direito → Colar, sem abrir Texto e arquivos, inclusive em
HTTP quando o navegador disponibiliza essa ação. Para copiar, selecione o texto
no terminal e use botão direito → Copiar; a seleção recebida é disponibilizada
no alvo nativo. Não se lê o clipboard local automaticamente. O RDP mantém seu
botão direito remoto; não foi substituído por um menu do navegador.

O teste integrado confirmou Ctrl+V recebendo exatamente `SSH-paste-test` no
servidor SSH descartável, além de renderização e reconexão. O evento contextmenu
é verificado como não cancelado; menus externos do navegador dependem de suas
permissões e plataforma e devem ser conferidos no navegador do operador.

## Fluxo implementado

- Com a tela remota focada, Ctrl+V usa o evento de colagem do navegador, envia
  texto pelo canal de clipboard e então aplica o atalho remoto. Não digita
  caracteres um a um. Também reconhece Shift+Insert. Limite de texto: 64 KiB.
- No RDP, Ctrl+C solicita a sincronização do próximo texto recebido com o
  clipboard local. Isso exige HTTPS confiável (ou localhost) e permissão do
  navegador. Recebimentos não solicitados ou em segundo plano não sobrescrevem
  o clipboard local. Não há leitura periódica do clipboard.
- Em HTTP por IP, a cópia remoto → local tem botão explícito no painel e
  seleção manual como alternativa. A colagem local → remoto usa o evento
  nativo e não requer a API de leitura assíncrona de clipboard.
- Arquivos e imagens que o navegador disponibilize no evento de colagem são
  enviados para a unidade temporária ConnectMe. Também há arrastar e soltar.
  Até 8 arquivos por operação, sequenciais, com os limites existentes da API.
- Screenshots colados são arquivos de imagem: não são colados como bitmap no
  Paint/Word. Ctrl+C de arquivos no Explorador não é garantido pelo navegador;
  use arrastar e soltar ou seletor quando ele não fornecer os arquivos.
- Remoto → local para arquivos continua por download no painel. Não há
  integração com a área de transferência de arquivos do Explorador local.
- O painel manual permanece disponível, com layout compacto e notificações
  na sessão. Uploads diretos não abrem automaticamente o painel.

O comportamento não equivale integralmente ao mstsc. No SSH, Ctrl+C mantém
seu significado de interrupção; a colagem é aplicada com Ctrl+Shift+V, e
múltiplas linhas pedem confirmação porque podem executar comandos. SFTP não
está habilitado. Não extrapolar homologação RDP para o terminal SSH.

## Segurança e ciclo de vida

O módulo `modules/identity/clipboard.js` gerencia gestos/foco; `transfers.js`
continua responsável por transporte, permissões, auditoria e limites. As APIs
de arquivos e o mecanismo RDP não foram substituídos. Somente a sessão focada
recebe a colagem. Trocar foco enquanto o pedido está pendente cancela o atalho
remoto. Encerrar remove listeners e dados locais da sessão. Permissões são
as mesmas quatro opções da conexão; não são ativadas automaticamente.

Arquivos continuam temporários, sem sobrescrita via upload, e são removidos
ao encerrar/reconectar/expirar/reiniciar. HTTPS é necessário também para
proteger credenciais e conteúdo em trânsito, não apenas para liberar APIs.

## Evidências e pendências

`tests/clipboard-browser.mjs` usa Ctrl+V nativo e clipboard real do Chromium
com dados fictícios; valida cópia de retorno solicitada, isolamento de foco,
negação de permissões, imagens como arquivos e remoção dos listeners.
Regressões existentes de sessões, upload/download e senha oculta passaram.

O teste `tests/rdp-clipboard-live.mjs` usa arquivos temporários próprios e senha
via stdin. Na validação local anterior, o fluxo passou duas vezes: texto nos
dois sentidos, leitura no Windows do arquivo enviado e download do arquivo salvo
pelo Windows. Isso valida transferência pela unidade, não clipboard de arquivos.

Para testar após o deploy: abra uma nova sessão RDP com permissões habilitadas,
abra um editor, copie texto local e use Ctrl+V na tela remota. Confira também
texto acentuado, múltiplas linhas, Ctrl+C remoto em HTTPS e uma captura de tela
colada como arquivo. Confirme o conteúdo na unidade `\\tsclient\ConnectMe`.
Escolher domínio/certificado HTTPS confiável continua pendente do operador.
Não iniciar a substituição do gateway antes de homologar esses fluxos.
