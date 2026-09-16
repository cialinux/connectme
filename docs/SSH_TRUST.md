# Identidade SSH no banco

As chaves públicas de servidores não são senhas nem chaves privadas. Ainda assim,
publicá-las junto com IPs expõe inventário desnecessário. Os arquivos
`base/ssh_known_hosts` e `config/ssh_known_hosts` foram removidos.

Ao abrir SSH, o painel solicita a identidade ao servidor sem enviar a senha.
No primeiro acesso a cada conexão/IP/porta, a chave é registrada no PostgreSQL
(confiança no primeiro uso, TOFU). Guacd recebe essa chave e a verifica antes
de autenticar. Uma mudança exige confirmação explícita pelo painel, mostrando
as impressões digitais anterior e atual; confirme por um canal independente.

`connections.ssh_host_keys` guarda a identidade e `ssh_host_key_events` registra
histórico e responsável. A migration é executada na inicialização. Não há
arquivo local ou alteração de repositório por dispositivo. Reiniciar preserva
a confiança. O primeiro acesso ainda pode sofrer interceptação; TOFU não
substitui confirmação independente da identidade em redes não confiáveis.

Instalações que usavam os arquivos antigos farão um novo registro no primeiro
acesso. Guarde uma cópia privada das chaves antigas para conferência antes de
atualizar. Não há importação silenciosa nem migração de segredos nesses arquivos.

Referência do parâmetro enviado ao gateway:
[Apache Guacamole — host-key](https://guacamole.apache.org/doc/gug/configuring-guacamole.html#ssh).
