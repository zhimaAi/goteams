// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"errors"

	"goteams/internal/app/goteams/define"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

const (
	MaxFavoriteContacts = 20
	MaxRecentContacts   = 20
)

var (
	ErrContactFavoriteLimit = errors.New(`favorite contact limit reached`)
	ErrContactInvalid       = errors.New(`invalid contact user`)
)

// ContactPreferences contains the current user's enterprise-wide contact preferences.
type ContactPreferences struct {
	FavoriteUserIds []int
	RecentUserIds   []int
}

// ListContactPreferences returns favorite and recently selected contacts for one tenant user.
func ListContactPreferences(adminId int, userId int) (ContactPreferences, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT preferences.contact_user_id,
		preferences.is_favorite, preferences.last_selected_at
		FROM user_contact_preferences preferences
		JOIN users contact ON contact.admin_id = preferences.admin_id
			AND contact.id = preferences.contact_user_id AND contact.status = 1
			AND contact.deleted_at = 0
		WHERE preferences.admin_id = $1 AND preferences.user_id = $2
		ORDER BY preferences.last_selected_at DESC, preferences.id DESC`, nil, adminId, userId)
	if err != nil {
		logs.Error("list contact preferences failed, err:%s", err.Error())
		return ContactPreferences{}, err
	}

	result := ContactPreferences{FavoriteUserIds: []int{}, RecentUserIds: []int{}}
	for _, row := range rows {
		contactUserId := cast.ToInt(row[`contact_user_id`])
		if cast.ToInt(row[`is_favorite`]) == 1 {
			result.FavoriteUserIds = append(result.FavoriteUserIds, contactUserId)
		}
		if cast.ToInt64(row[`last_selected_at`]) > 0 && len(result.RecentUserIds) < MaxRecentContacts {
			result.RecentUserIds = append(result.RecentUserIds, contactUserId)
		}
	}
	return result, nil
}

// UpdateFavoriteContact adds or removes an enterprise-wide favorite contact.
func UpdateFavoriteContact(adminId int, userId int, contactUserId int, favorite bool) error {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Serialize favorite updates for this user so concurrent requests cannot exceed the limit.
	owner, err := msql.RawValues(define.Postgres, `SELECT id FROM users
		WHERE admin_id = $1 AND id = $2 FOR UPDATE`, tx, adminId, userId)
	if err != nil {
		return err
	}
	if len(owner) == 0 {
		return ErrContactInvalid
	}
	contact, err := msql.RawValues(define.Postgres, `SELECT id FROM users
		WHERE admin_id = $1 AND id = $2 AND status = 1 AND deleted_at = 0`, tx, adminId, contactUserId)
	if err != nil {
		return err
	}
	if len(contact) == 0 {
		return ErrContactInvalid
	}

	now := tool.Time2Int()
	if favorite {
		countRows, countErr := msql.RawValues(define.Postgres, `SELECT COUNT(*) AS total
			FROM user_contact_preferences preferences
			JOIN users contact ON contact.admin_id = preferences.admin_id
				AND contact.id = preferences.contact_user_id AND contact.status = 1
			AND contact.deleted_at = 0
			WHERE preferences.admin_id = $1 AND preferences.user_id = $2
				AND preferences.is_favorite = 1`, tx, adminId, userId)
		if countErr != nil {
			return countErr
		}
		existing, existingErr := msql.RawValues(define.Postgres, `SELECT is_favorite FROM user_contact_preferences
			WHERE admin_id = $1 AND user_id = $2 AND contact_user_id = $3`, tx, adminId, userId, contactUserId)
		if existingErr != nil {
			return existingErr
		}
		alreadyFavorite := len(existing) > 0 && cast.ToInt(existing[0][`is_favorite`]) == 1
		if !alreadyFavorite && len(countRows) > 0 && cast.ToInt(countRows[0][`total`]) >= MaxFavoriteContacts {
			return ErrContactFavoriteLimit
		}
	}

	isFavorite := 0
	if favorite {
		isFavorite = 1
	}
	if _, err = msql.RawExec(define.Postgres, `INSERT INTO user_contact_preferences
		(admin_id, user_id, contact_user_id, is_favorite, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5)
		ON CONFLICT (admin_id, user_id, contact_user_id)
		DO UPDATE SET is_favorite = EXCLUDED.is_favorite, updated_at = EXCLUDED.updated_at`, tx,
		adminId, userId, contactUserId, isFavorite, now); err != nil {
		return err
	}
	return tx.Commit()
}

// RecordRecentContacts records the final assignee selection after a successful work-item update.
func RecordRecentContacts(adminId int, userId int, contactUserIds []int) error {
	ids := uniquePositiveContactIds(contactUserIds)
	if len(ids) == 0 {
		return nil
	}
	validRows, err := msql.RawValues(define.Postgres, `SELECT id FROM users
		WHERE admin_id = $1 AND status = 1 AND deleted_at = 0 AND id = ANY($2)`, nil, adminId, pq.Array(ids))
	if err != nil {
		return err
	}
	if len(validRows) != len(ids) {
		return ErrContactInvalid
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	now := tool.Time2Int()
	for index, contactUserId := range ids {
		selectedAt := now - (len(ids) - 1 - index)
		if _, err = msql.RawExec(define.Postgres, `INSERT INTO user_contact_preferences
			(admin_id, user_id, contact_user_id, last_selected_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $4, $4)
			ON CONFLICT (admin_id, user_id, contact_user_id)
			DO UPDATE SET last_selected_at = EXCLUDED.last_selected_at, updated_at = EXCLUDED.updated_at`, tx,
			adminId, userId, contactUserId, selectedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func uniquePositiveContactIds(ids []int) []int {
	seen := make(map[int]struct{}, len(ids))
	result := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	if len(result) > MaxRecentContacts {
		result = result[len(result)-MaxRecentContacts:]
	}
	return result
}
