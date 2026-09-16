# Acesso de rede

O padrão não inclui NetworkPolicy Kubernetes nem restrições de IP no Ingress.
O Deployment e o Compose deixam `CONNECTME_REMOTE_DENY_CIDRS` vazio.
A aplicação continua exigindo login, autorização e cadastro válido da rede/host.
Para permitir qualquer IPv4/IPv6 numa localização, o administrador pode cadastrar
as redes correspondentes no painel; não há IP de laboratório fixado no código.

Acessar o site não comprova conectividade aos destinos. Verifique rota, NAT,
firewall e porta TCP a partir do guacd. SSH registra confiança no banco;
RDP ignora certificados globalmente quando `CONNECTME_RDP_IGNORE_CERT=true`.
O próprio servidor também pode ser cadastrado, sem exceção específica por IP.
