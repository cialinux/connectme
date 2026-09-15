# Acesso de rede — 2026-09-15

Por solicitação do operador, a NetworkPolicy de sessão permite toda saída
(`egress: [{}]`), sem lista de IPs/portas. Os Ingress dev/pro não têm allowlist
de origem: o painel HTTPS pode ser acessado de qualquer rede, com login/RBAC.
Guacd e PostgreSQL continuam sem exposição pública e com entrada isolada.

A aplicação mantém bloqueio de destinos internos do cluster e link-local:
10.244.0.0/16, 10.43.0.0/16, 169.254.0.0/16. O bloqueio especial do host de
laboratório 192.168.1.248 foi removido. Cadastro/autorização das redes e hosts,
validação de destinos, chaves SSH e certificados RDP não foram desativados.

Diagnóstico de 94.62.108.14:33891: a política antiga já permitia essa combinação.
O teste TCP no guacd retornou Host is unreachable; tentativas anteriores também
registraram Server refused connection (wrong security type?). Liberar egress
não comprova NAT, firewall ou negociação RDP corretos. A exceção de certificado
atualmente ativa cobre apenas 192.168.1.200/32, não o IP público. Não foi ampliada
sem validar o certificado/destino. NLA continua obrigatório.

Após o operador publicar/reconciliar, testar:

```sh
kubectl --kubeconfig "$HOME/.kube/config-staging" -n connectme-pro exec deployment/connectme -c guacd -- nc -z -v -w 5 94.62.108.14 33891
kubectl --kubeconfig "$HOME/.kube/config-staging" -n connectme-pro logs deployment/connectme -c guacd --since=5m
```

Se TCP continuar inacessível, conferir NAT TCP 33891 para o Windows/porta RDP,
firewalls e rota de retorno. O IP de saída do worker pode diferir do IP do Ingress.
Não foi aplicada alteração no cluster durante esta preparação.
