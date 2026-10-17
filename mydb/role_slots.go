package mydb

import (
	"database/sql"
	"errors"
	"log/slog"
)

// RoleSlot is one extra-role position on a game. UserID 0 means a free-form name.
type RoleSlot struct {
	ID, GameID, RoleID, Slot, UserID                int
	RoleName, FreeName, Status, UserName, UserEmail string
}

const roleSlotSelect = `SELECT gr.id, gr.game_id, gr.role_id, gr.slot, COALESCE(gr.user_id,0), gr.free_name, gr.status,
	r.name, COALESCE(u.name,''), COALESCE(u.email,'')
	FROM game_role_slots gr JOIN extra_roles r ON r.id = gr.role_id LEFT JOIN users u ON u.id = gr.user_id`

func (me *MyDB) queryRoleSlots(where string, args ...any) []RoleSlot {
	rows, err := me.DB.Query(roleSlotSelect+" "+where, args...)
	if err != nil {
		slog.Error("queryRoleSlots", "err", err)
		return nil
	}
	defer rows.Close()
	var out []RoleSlot
	for rows.Next() {
		var s RoleSlot
		if err := rows.Scan(&s.ID, &s.GameID, &s.RoleID, &s.Slot, &s.UserID, &s.FreeName, &s.Status, &s.RoleName, &s.UserName, &s.UserEmail); err != nil {
			slog.Error("queryRoleSlots scan", "err", err)
			continue
		}
		out = append(out, s)
	}
	return out
}

func (me *MyDB) GameRoleSlots(gameID int) []RoleSlot {
	return me.queryRoleSlots(`WHERE gr.game_id=$1 ORDER BY lower(r.name), gr.role_id, gr.slot`, gameID)
}

func (me *MyDB) RoleSlotsForUser(userID int) []RoleSlot {
	return me.queryRoleSlots(`WHERE gr.user_id=$1 ORDER BY gr.game_id, lower(r.name), gr.slot`, userID)
}

func (me *MyDB) AssignRoleSlot(gameID, roleID, slot, userID int, freeName string) error {
	tx, err := me.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if userID > 0 {
		if _, err := tx.Exec(`SELECT 1 FROM users WHERE id=$1 FOR UPDATE`, userID); err != nil {
			return err
		}
		var dup bool
		if err := tx.QueryRow(`SELECT EXISTS (SELECT 1 FROM game_umpires WHERE game_id=$1 AND user_id=$2 AND status<>'declined')
			OR EXISTS (SELECT 1 FROM game_role_slots WHERE game_id=$1 AND user_id=$2 AND status<>'declined' AND NOT (role_id=$3 AND slot=$4))`,
			gameID, userID, roleID, slot).Scan(&dup); err != nil {
			return err
		}
		if dup {
			return ErrUmpireDuplicate
		}
		var start sql.NullTime
		if err := tx.QueryRow(`SELECT start_time FROM games WHERE id=$1`, gameID).Scan(&start); err != nil {
			return err
		}
		if start.Valid && umpireConflict(tx, userID, start.Time, gameID) {
			return ErrUmpireConflict
		}
		_, err = tx.Exec(`INSERT INTO game_role_slots (game_id, role_id, slot, user_id, free_name, status) VALUES ($1,$2,$3,$4,'','pending')
			ON CONFLICT (game_id, role_id, slot) DO UPDATE SET user_id=$4, free_name='', status='pending'`, gameID, roleID, slot, userID)
	} else {
		_, err = tx.Exec(`INSERT INTO game_role_slots (game_id, role_id, slot, user_id, free_name, status) VALUES ($1,$2,$3,NULL,$4,'accepted')
			ON CONFLICT (game_id, role_id, slot) DO UPDATE SET user_id=NULL, free_name=$4, status='accepted'`, gameID, roleID, slot, freeName)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (me *MyDB) ClearRoleSlot(gameID, roleID, slot int) {
	if _, err := me.DB.Exec(`DELETE FROM game_role_slots WHERE game_id=$1 AND role_id=$2 AND slot=$3`, gameID, roleID, slot); err != nil {
		slog.Error("ClearRoleSlot", "err", err)
	}
}

func (me *MyDB) RespondRoleSlot(gameID, roleID, slot, userID int, accept bool) error {
	status := "declined"
	if accept {
		status = "accepted"
	}
	res, err := me.DB.Exec(`UPDATE game_role_slots SET status=$1 WHERE game_id=$2 AND role_id=$3 AND slot=$4 AND user_id=$5 AND status<>'declined'`,
		status, gameID, roleID, slot, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var declined bool
		me.DB.QueryRow(`SELECT EXISTS (SELECT 1 FROM game_role_slots WHERE game_id=$1 AND role_id=$2 AND slot=$3 AND user_id=$4 AND status='declined')`,
			gameID, roleID, slot, userID).Scan(&declined)
		if declined {
			return ErrUmpireSlotDeclined
		}
		return errors.New("slot not assigned to this user")
	}
	return nil
}

func (me *MyDB) ResetRoleSlotsPending(gameID int) []RoleSlot {
	affected := me.queryRoleSlots(`WHERE gr.game_id=$1 AND gr.user_id IS NOT NULL AND gr.status='accepted' ORDER BY lower(r.name), gr.slot`, gameID)
	if len(affected) == 0 {
		return nil
	}
	if _, err := me.DB.Exec(`UPDATE game_role_slots SET status='pending' WHERE game_id=$1 AND user_id IS NOT NULL AND status='accepted'`, gameID); err != nil {
		slog.Error("ResetRoleSlotsPending", "err", err)
		return nil
	}
	return affected
}
