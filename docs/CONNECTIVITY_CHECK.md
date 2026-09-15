# Verificação de acesso — 2026-09-15

- SSH 94.62.108.14:22220: chave RSA coincidiu com a chave previamente aprovada
  de 192.168.1.248:22. Autenticação com a credencial atualmente cadastrada passou
  via kubectl/guacd, sem abrir shell ou executar comandos no destino. Alias
  público adicionado aos arquivos config/ssh_known_hosts e base/ssh_known_hosts.
- RDP 94.62.108.14:33891: teste com a credencial atualmente cadastrada retornou
  519. Teste TCP no pod retornou Host is unreachable. A senha não foi validada;
  não há evidência suficiente para atribuir a falha à senha. Verificar NAT TCP
  33891 para Windows:3389, firewall e rota de retorno.
- Ingress ativo ainda continha whitelist 94.62.108.14/32. Os manifests dev/pro
  agora definem explicitamente 0.0.0.0/0,::/0 para substituir o valor antigo.
  HTTPS e autenticação permanecem. Da origem de diagnóstico o site respondeu 200;
  a restrição ativa explica o bloqueio de outras origens.

Os dois overlays renderizaram e passaram no dry-run cliente. Nenhuma credencial
foi gravada no projeto, nenhum apply/patch foi executado e nenhuma configuração
do Windows/Linux foi alterada. O ConfigMap de confiança terá novo hash no deploy,
recriando o pod e encerrando sessões atuais.

IMPORTANTE: atualizar somente base não atualiza o Ingress, definido em overlays.
O Ingress observado possui last-applied-configuration de kubectl e não as
anotações Helm da release da base. Confirme que ele entra no seu deploy ou,
como operador, atualize especificamente o manifesto:

```sh
kubectl --kubeconfig "$HOME/.kube/config-staging" -n connectme-pro apply -f overlays/pro/ingress.yaml
```

Esse comando foi documentado, não executado pelo assistente.
