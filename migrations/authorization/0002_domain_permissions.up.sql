INSERT INTO authz.permissions(name,description) VALUES
('locations.read','Consultar localizações'),('locations.manage','Gerir localizações e redes'),
('hosts.read','Consultar hosts'),('hosts.manage','Gerir hosts'),
('credentials.read','Consultar metadados de credenciais'),('credentials.manage','Gerir credenciais'),
('connections.read','Consultar conexões'),('connections.manage','Gerir conexões') ON CONFLICT DO NOTHING;
INSERT INTO authz.role_permissions(role_id,permission) SELECT '00000000-0000-4000-8000-000000000001',name FROM authz.permissions ON CONFLICT DO NOTHING;
