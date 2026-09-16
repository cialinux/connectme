package connections

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

var ErrSSHTrustConflict = errors.New("a identidade SSH mudou; atualize e confirme novamente")

func (s *Service) SSHKey(ctx context.Context, id, address string, port int) (string, error) {
	var key string
	err := s.pool.QueryRow(ctx, "SELECT public_key FROM connections.ssh_host_keys WHERE connection_id=$1 AND address=$2 AND port=$3", id, address, port).Scan(&key)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return key, err
}

// Compare-and-swap plus history in one transaction. A simultaneous first
// connection or approval must never silently replace a different host key.
func (s *Service) SaveSSHKey(ctx context.Context, id, address string, port int, previous, key, actor string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if previous == "" {
		tag, e := tx.Exec(ctx, "INSERT INTO connections.ssh_host_keys(connection_id,address,port,public_key,updated_by) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING", id, address, port, key, actor)
		if e != nil {
			return e
		}
		if tag.RowsAffected() != 1 {
			return ErrSSHTrustConflict
		}
	} else {
		tag, e := tx.Exec(ctx, "UPDATE connections.ssh_host_keys SET public_key=$5,updated_by=$6,updated_at=now() WHERE connection_id=$1 AND address=$2 AND port=$3 AND public_key=$4", id, address, port, previous, key, actor)
		if e != nil {
			return e
		}
		if tag.RowsAffected() != 1 {
			return ErrSSHTrustConflict
		}
	}
	_, err = tx.Exec(ctx, "INSERT INTO connections.ssh_host_key_events(connection_id,address,port,previous_key,public_key,actor_id) VALUES($1,$2,$3,$4,$5,$6)", id, address, port, previous, key, actor)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
