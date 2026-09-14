ALTER TABLE connections.connections
 ADD COLUMN clipboard_copy_enabled boolean NOT NULL DEFAULT false,
 ADD COLUMN clipboard_paste_enabled boolean NOT NULL DEFAULT false,
 ADD COLUMN file_upload_enabled boolean NOT NULL DEFAULT false,
 ADD COLUMN file_download_enabled boolean NOT NULL DEFAULT false;
UPDATE connections.connections SET
 clipboard_copy_enabled=clipboard_enabled,
 clipboard_paste_enabled=clipboard_enabled,
 file_upload_enabled=file_transfer_enabled,
 file_download_enabled=file_transfer_enabled;
